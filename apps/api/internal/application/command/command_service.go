package command

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/dto"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/application/shared"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/command"
	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/domain/device"
)

const DefaultCommandTTL = 5 * time.Minute
const MaxPendingCommandsPerDevice = 50

// ErrTooManyPendingCommands is returned when a device has too many pending commands.
var ErrTooManyPendingCommands = errors.New("device has too many pending commands")

// Service handles command operations.
type Service struct {
	commandRepo command.Repository
	deviceRepo  device.Repository
	publisher   StatusPublisher
}

// NewService creates a new CommandService.
func NewService(commandRepo command.Repository, deviceRepo device.Repository) *Service {
	return &Service{
		commandRepo: commandRepo,
		deviceRepo:  deviceRepo,
	}
}

// SendCommand sends a command to a device.
func (s *Service) SendCommand(ctx context.Context, req *dto.SendCommandRequest) (*dto.SendCommandResponse, error) {
	// Check if device exists.
	_, err := s.deviceRepo.FindByID(ctx, req.DeviceID)
	if err != nil {
		if errors.Is(err, device.ErrNotFound) {
			return nil, application.ErrDeviceNotFound
		}

		return nil, err
	}
	pendingCount, err := s.commandRepo.CountPendingByDevice(ctx, req.DeviceID)
	if err != nil {
		return nil, err
	}
	if pendingCount >= MaxPendingCommandsPerDevice {
		return nil, ErrTooManyPendingCommands
	}

	// Generate dispatch ID for idempotency.
	id := shared.GenerateID()
	dispatchID := id

	// Check for duplicate dispatch (idempotency).
	if req.DispatchID != "" {
		existing, findErr := s.commandRepo.FindByDispatchID(ctx, req.DeviceID, req.DispatchID)
		if findErr != nil && findErr != command.ErrNotFound {
			return nil, findErr
		}

		if existing != nil {
			// Return existing command (idempotent).
			return &dto.SendCommandResponse{
				CommandID:  existing.ID,
				DeviceID:   existing.DeviceID,
				DispatchID: existing.DispatchID,
				Status:     string(existing.Status),
				CreatedAt:  existing.CreatedAt,
			}, nil
		}

		dispatchID = req.DispatchID
	}

	// Encode args if provided.
	var args []byte
	if req.Args != nil {
		args, err = json.Marshal(req.Args)
		if err != nil {
			return nil, err
		}
	}

	// Generate command ID.
	cmdID := shared.GenerateID()

	now := time.Now()
	expiresAt := now.Add(DefaultCommandTTL)
	cmd := &command.Command{
		ID:         cmdID,
		DeviceID:   req.DeviceID,
		DispatchID: dispatchID,
		Command:    req.Command,
		Args:       args,
		Status:     command.StatusPending,
		RetryCount: 0,
		MaxRetries: 5, // Default max retries for outbox pattern.
		CreatedAt:  now,
		UpdatedAt:  now,
		ExpiresAt:  &expiresAt,
	}

	if err := s.commandRepo.Create(ctx, cmd); err != nil {
		return nil, err
	}

	return &dto.SendCommandResponse{
		CommandID:  cmd.ID,
		DeviceID:   cmd.DeviceID,
		DispatchID: cmd.DispatchID,
		Status:     string(cmd.Status),
		CreatedAt:  cmd.CreatedAt,
	}, nil
}

// GetCommandStatus gets the status of a command.
func (s *Service) GetCommandStatus(ctx context.Context, commandID string) (*dto.CommandStatusResponse, error) {
	cmd, err := s.commandRepo.FindByID(ctx, commandID)
	if err != nil {
		if errors.Is(err, command.ErrNotFound) {
			return nil, application.ErrCommandNotFound
		}

		return nil, err
	}

	return &dto.CommandStatusResponse{
		CommandID:   cmd.ID,
		DeviceID:    cmd.DeviceID,
		Command:     cmd.Command,
		Status:      string(cmd.Status),
		DeliveredAt: cmd.DeliveredAtTime(),
		CompletedAt: cmd.CompletedAtTime(),
		CreatedAt:   cmd.CreatedAt,
		UpdatedAt:   cmd.UpdatedAt,
	}, nil
}

// GetPendingCommands gets pending commands for a device.
func (s *Service) GetPendingCommands(ctx context.Context, deviceID string) ([]dto.CommandResponse, error) {
	cmds, err := s.commandRepo.FindPendingByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, err
	}

	response := make([]dto.CommandResponse, len(cmds))
	for i, cmd := range cmds {
		response[i] = dto.CommandResponse{
			ID:        cmd.ID,
			DeviceID:  cmd.DeviceID,
			Command:   cmd.Command,
			Args:      cmd.Args,
			Status:    string(cmd.Status),
			CreatedAt: cmd.CreatedAt,
			UpdatedAt: cmd.UpdatedAt,
		}
	}

	return response, nil
}

// MarkDelivered marks a command as delivered.
func (s *Service) MarkDelivered(ctx context.Context, commandID string) error {
	cmd, err := s.commandRepo.FindByID(ctx, commandID)
	if err != nil {
		return err
	}

	now := time.Now().UnixMilli()
	cmd.Status = command.StatusDelivered
	cmd.DeliveredAt = &now
	cmd.UpdatedAt = time.Now()

	return s.commandRepo.Update(ctx, cmd)
}

// MarkCompleted marks a command as completed by dispatch ID, recording the
// device-reported output. It is the counterpart of MarkDelivered and is the
// entry point a device's command result travels through.
func (s *Service) MarkCompleted(ctx context.Context, dispatchID, result string) error {
	return s.applyTerminal(ctx, dispatchID, true, result)
}

// MarkFailed marks a command as failed by dispatch ID with an error message.
func (s *Service) MarkFailed(ctx context.Context, dispatchID, failureReason string) error {
	return s.applyTerminal(ctx, dispatchID, false, failureReason)
}

// applyTerminal loads the command, applies the terminal transition through the
// domain, and persists it. The domain enforces the lifecycle (an already
// terminal command is not resurrected by a late report) and keeps result and
// failure_reason distinct.
func (s *Service) applyTerminal(ctx context.Context, dispatchID string, success bool, output string) error {
	cmd, err := s.commandRepo.FindByDispatchIDOnly(ctx, dispatchID)
	if err != nil {
		if errors.Is(err, command.ErrNotFound) {
			return application.ErrCommandNotFound
		}

		return err
	}

	return s.applyTerminalTo(ctx, cmd, success, output)
}

// ReportResult records a device-reported terminal outcome for a command that
// belongs to deviceID. The ownership check matters: a device must not be able to
// complete or fail a command dispatched to a different device by guessing a
// dispatch ID.
func (s *Service) ReportResult(ctx context.Context, deviceID, dispatchID string, success bool, output string) error {
	cmd, err := s.commandRepo.FindByDispatchIDOnly(ctx, dispatchID)
	if err != nil {
		if errors.Is(err, command.ErrNotFound) {
			return application.ErrCommandNotFound
		}

		return err
	}

	if cmd.DeviceID != deviceID {
		return application.ErrCommandNotFound
	}

	return s.applyTerminalTo(ctx, cmd, success, output)
}

// applyTerminalTo applies the terminal transition to a loaded command, persists
// it, and publishes the new status. Applying the same terminal status twice is a
// no-op, so a retried device report does not produce a spurious error.
func (s *Service) applyTerminalTo(ctx context.Context, cmd *command.Command, success bool, output string) error {
	changed, err := cmd.ApplyResult(success, output, time.Now())
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}

	if err := s.commandRepo.Update(ctx, cmd); err != nil {
		return err
	}

	if s.publisher != nil {
		s.publisher.PublishCommandStatusForDevice(ctx, cmd.DispatchID, cmd.DeviceID, string(cmd.Status))
	}

	return nil
}

// SetStatusPublisher installs the hook used to fan a terminal command status out
// to realtime subscribers. It is optional so the service stays usable without a
// hub (tests, CLI paths); a nil publisher simply skips the notification.
func (s *Service) SetStatusPublisher(p StatusPublisher) {
	s.publisher = p
}

// StatusPublisher publishes a command status change to realtime subscribers.
type StatusPublisher interface {
	PublishCommandStatusForDevice(ctx context.Context, dispatchID, deviceID, status string)
}

// CancelCommand cancels a pending command.
func (s *Service) CancelCommand(ctx context.Context, commandID string) error {
	cmd, err := s.commandRepo.FindByID(ctx, commandID)
	if err != nil {
		return err
	}

	if !cmd.IsPending() {
		return application.ErrCommandFailed // Can only cancel pending commands.
	}

	return s.commandRepo.Delete(ctx, commandID)
}

// RetryCommand retries a failed or pending command by creating a new command with the same parameters.
func (s *Service) RetryCommand(ctx context.Context, dispatchID string) (*dto.SendCommandResponse, error) {
	// Find the original command by dispatch ID only (globally unique).
	original, err := s.commandRepo.FindByDispatchIDOnly(ctx, dispatchID)
	if err != nil {
		return nil, err
	}

	// Create a new command with new IDs.
	cmdReq := &dto.SendCommandRequest{
		DeviceID: original.DeviceID,
		Command:  original.Command,
		Args:     original.Args,
	}

	return s.SendCommand(ctx, cmdReq)
}

// GetCommandByDispatchID retrieves a command by dispatch ID.
func (s *Service) GetCommandByDispatchID(ctx context.Context, dispatchID string) (*dto.CommandStatusResponse, error) {
	cmd, err := s.commandRepo.FindByDispatchIDOnly(ctx, dispatchID)
	if err != nil {
		if errors.Is(err, command.ErrNotFound) {
			return nil, application.ErrCommandNotFound
		}

		return nil, err
	}

	return &dto.CommandStatusResponse{
		CommandID:   cmd.ID,
		DispatchID:  cmd.DispatchID,
		DeviceID:    cmd.DeviceID,
		Command:     cmd.Command,
		Status:      string(cmd.Status),
		DeliveredAt: cmd.DeliveredAtTime(),
		CompletedAt: cmd.CompletedAtTime(),
		CreatedAt:   cmd.CreatedAt,
		UpdatedAt:   cmd.UpdatedAt,
	}, nil
}

// CancelCommandByDispatchID cancels a pending command by dispatch ID.
func (s *Service) CancelCommandByDispatchID(ctx context.Context, dispatchID string) error {
	cmd, err := s.commandRepo.FindByDispatchIDOnly(ctx, dispatchID)
	if err != nil {
		return err
	}

	if !cmd.IsPending() {
		return application.ErrCommandFailed // Can only cancel pending commands.
	}

	return s.commandRepo.Delete(ctx, cmd.ID)
}

// CommandRepo returns the command repository for use by other services.
func (s *Service) CommandRepo() command.Repository {
	return s.commandRepo
}
