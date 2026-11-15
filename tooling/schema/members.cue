package vyzorix

// OrganizationMember.
#OrganizationMember: {
        id: string
        organization_id: string
        user_id: string
        role: string
        invited_by?: string
        joined_at?: int64
        removed_at?: int64
        status: string
        user_name?: string
        user_email?: string
}

// OrganizationMemberListResult.
#OrganizationMemberListResult: {
        members: [...#OrganizationMember]
}

// UpdateMemberRoleRequest.
#UpdateMemberRoleRequest: {
        role: string
}
