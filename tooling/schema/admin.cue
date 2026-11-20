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

// OrgUser is one member of the current organisation: the account's profile
// facts plus the membership facts (role, status, joined). There is no
// deployment-wide equivalent.
#OrgUser: {
        id: string
        member_id: string
        email: string
        name: string
        role: string
        status: string
        joined_at: int64
        avatar_url?: string
        created_at: int64
        last_seen_at?: int64
        mfa_enabled: bool
        email_verified: bool
        disabled: bool
}

// OrgUserListResult.
#OrgUserListResult: {
        users: [...#OrgUser]
        total: int
        page: int
        limit: int
        total_pages: int
}

// OrgUserStatsResult is the member count per role plus the pending invitation
// count for one organisation.
#OrgUserStatsResult: {
        by_role: {[string]: int}
        total: int
        pending_invites: int
}

// AdminUserSessionListResult.
#AdminUserSessionListResult: {
        sessions: [...#SessionInfo]
        total: int
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
