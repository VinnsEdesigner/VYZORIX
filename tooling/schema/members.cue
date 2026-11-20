package vyzorix

// OrganizationMember. The membership facts (role, status, joined) are
// per-organisation; avatar_url and last_seen are the account's, joined in so the
// members table can render an avatar and a last-seen column without a second
// request.
#OrganizationMember: {
        id: string
        organization_id: string
        user_id: string
        role: string
        status: string
        invited_by?: string
        joined_at?: int64
        removed_at?: int64
        suspended_at?: int64
        user_name?: string
        user_email?: string
        avatar_url?: string
        last_seen?: int64
}

// OrganizationMemberListResult.
#OrganizationMemberListResult: {
        members: [...#OrganizationMember]
}

// UpdateMemberRoleRequest.
#UpdateMemberRoleRequest: {
        role: string
}
