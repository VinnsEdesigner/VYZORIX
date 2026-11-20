package vyzorix

// CreateServiceAccountRequest.
#CreateServiceAccountRequest: {
        name: string
}

// FixedRoleGrant.
#FixedRoleGrant: {
        role: string
        scope: string
}

// CreateServiceAccountTokenRequest.
#CreateServiceAccountTokenRequest: {
        expires_at?: string
        service_id: string
        name: string
        basic_role?: string
        roles?: [...#FixedRoleGrant]
}

// UpdateServiceAccountTokenRolesRequest.
#UpdateServiceAccountTokenRolesRequest: {
        basic_role?: string
        roles?: [...#FixedRoleGrant]
}

// RotateServiceAccountTokenRequest.
#RotateServiceAccountTokenRequest: {
        expires_at?: string
        name: string
        basic_role?: string
        roles?: [...#FixedRoleGrant]
}

// UpdateServiceAccountRequest. `reason` is recorded with a state change so a
// disabled account can explain itself; it is ignored on a rename.
#UpdateServiceAccountRequest: {
        name?: string
        enabled?: bool
        reason?: string
}

// ServiceAccount.
#ServiceAccount: {
        created_at: int64
        updated_at: int64
        id: string
        org_id: string
        name: string
        enabled: bool
        token_count: int
        avatar_url: string
        // State facts: who last changed `enabled`, when, and why. Present only
        // while disabled; re-enabling clears them. The *ByName fields are
        // resolved on read from the stored user ids.
        disabled_at?: int64
        disabled_by?: string
        disabled_by_name?: string
        enabled_by?: string
        enabled_by_name?: string
        disabled_reason?: string
}

// ServiceAccountToken.
#ServiceAccountToken: {
        created_at: int64
        expires_at?: int64
        revoked_at?: int64
        last_used_at?: int64
        id: string
        service_id: string
        name: string
        key_prefix: string
        scopes: [...string]
        basic_role?: string
        roles: [...#FixedRoleGrant]
        valid: bool
}

// ServiceAccountListResult.
#ServiceAccountListResult: {
        service_accounts: [...#ServiceAccount]
}

// ServiceAccountTokenListResult.
#ServiceAccountTokenListResult: {
        tokens: [...#ServiceAccountToken]
}

// ServiceAccountTokenCreated.
#ServiceAccountTokenCreated: {
        secret: string
}

// ServiceAccountTokenRotated.
#ServiceAccountTokenRotated: {
        secret: string
}

// FixedRoleInfo.
#FixedRoleInfo: {
        id: string
        group: string
        access: string
}

// FixedRoleCatalogResult.
#FixedRoleCatalogResult: {
        roles: [...#FixedRoleInfo]
        org_roles: [...string]
        scopes: [...string]
}
