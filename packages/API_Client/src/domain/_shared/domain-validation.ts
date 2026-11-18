export interface ValidationFieldError {
  field: string;
  message: string;
}

export interface ValidationResult {
  isValid: boolean;
  errors: Record<string, string[]>;
}

export function validResult(): ValidationResult {
  return { isValid: true, errors: {} };
}

export function invalidResult(errors: Record<string, string[]>): ValidationResult {
  return { isValid: Object.keys(errors).length === 0, errors };
}

export function fieldErrorList(result: ValidationResult): ValidationFieldError[] {
  return Object.entries(result.errors).flatMap(([field, messages]) =>
    messages.map((message) => ({ field, message }))
  );
}

const IMEI_REGEX = /^\d{15}$/;

export function validateIMEI(imei: string): ValidationResult {
  const errors: Record<string, string[]> = {};
  if (!imei) {
    errors.imei = ["IMEI is required"];
  } else if (!IMEI_REGEX.test(imei)) {
    errors.imei = ["IMEI must be 15 digits"];
  }
  return { isValid: Object.keys(errors).length === 0, errors };
}
