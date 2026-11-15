package vyzorix

// MFAStatusResult.
#MFAStatusResult: {
        mfa_enabled: bool
}

// MFAEnrollResult.
#MFAEnrollResult: {
        secret: string
        uri: string
}

// MFAVerifySetupRequest.
#MFAVerifySetupRequest: {
        code: string
        token: string
}

// MFAEnableRequest.
#MFAEnableRequest: {
        code: string
        token: string
}

// MFAEnableResult.
#MFAEnableResult: {
        backup_codes: [...string]
        success: bool
}

// MFADisableRequest.
#MFADisableRequest: {
        code: string
}

// MFABackupCodeRequest.
#MFABackupCodeRequest: {
        code: string
}

// MFABackupCodeResult.
#MFABackupCodeResult: {
        valid: bool
}

// MFARegenerateResult.
#MFARegenerateResult: {
        backup_codes: [...string]
}

// MFAVerifyRequest.
#MFAVerifyRequest: {
        user_id: string
        code: string
}

// MFAVerifyResult.
#MFAVerifyResult: {
        session_id: string
        access_token: string
        refresh_token: string
        signing_key: string
        user: #MFAUser
        expires_at: int64
        success: bool
}

// MFAUser.
#MFAUser: {
        id: string
        email: string
        name: string
        role: string
        mfa_enabled: bool
}

// MFAVerifiedResult.
#MFAVerifiedResult: {
        verified: bool
}
