package vyzorix

// TopUserStat.
#TopUserStat: {
        user_id: string
        user_name: string
        total_requests: int64
        active_key_count: int
}

// UserAPIKeyStatsResult.
#UserAPIKeyStatsResult: {
        user_id: string
        total_keys: int64
        active_keys: int64
        revoked_keys: int64
        keys_created_this_month: int
        monthly_limit: int
}

// AdminUser.
#AdminUser: {
        id: string
        email: string
        name: string
        role?: string
        created_at: int64
        updated_at: int64
        mfa_enabled: bool
        email_verified: bool
}

// AdminUserListResult.
#AdminUserListResult: {
        users: [...#AdminUser]
        total: int
}

// CreateUserRequest.
#CreateUserRequest: {
        email: string
        password: string
        name: string
        role?: string
}

// UserThresholds.
#UserThresholds: {
        riskWarn?: int
        riskCrit?: int
        thermalWarn?: int
        thermalCrit?: int
        bufferWarn?: int
        bufferCrit?: int
}

// UserSettingsResult.
#UserSettingsResult: {
        notifications?: #NotificationSettings
        client?: #ClientSettings
        thresholds?: #UserThresholds
}

// UserSettingsResultLegacy.
#UserSettingsResultLegacy: {
        client?: #ClientSettings
        id: string
        email: string
        name: string
        avatar_url?: string
        role?: string
        mfa_enabled: bool
        email_verified: bool
}

// AdminClientActionResult.
#AdminClientActionResult: {
        success: bool
        client_id?: string
}
