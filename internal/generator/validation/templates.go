package validation

// useValidationRules is framework-agnostic — identical for Quasar and Nuxt.
const useValidationRules = `// A validation rule is a function that receives a field value
// and returns either ` + "`true`" + ` (valid) or an error string.
export type ValidationRule = (value: string) => true | string

export const required: ValidationRule = (value) =>
  value.trim().length > 0 || 'This field is required'

export const minString: ValidationRule = (value) =>
  value.trim().length >= 2 || 'Must be at least 2 characters'

export const email: ValidationRule = (value) =>
  /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(value) || 'Enter a valid email address'

export const password: ValidationRule = (value) =>
  /^(?=.*[A-Z])(?=.*\d).{8,}$/.test(value) ||
  'Password must be at least 8 characters, include an uppercase letter and a number'

// Factory — receives a getter so it can read another field's value at validation time
export const confirmPassword =
  (getPassword: () => string): ValidationRule =>
  (value) =>
    value === getPassword() || 'Passwords do not match'
`

// useForm is framework-agnostic — identical for Quasar and Nuxt.
const useForm = `import { reactive, computed } from 'vue'
import type { ValidationRule } from './useValidationRules'

interface FieldConfig {
  rules?: ValidationRule[]
}

type FieldsConfig = Record<string, FieldConfig>
type FormData<T extends FieldsConfig> = { [K in keyof T]: string }
type FormErrors<T extends FieldsConfig> = { [K in keyof T]: string }

// configFactory receives the reactive form data so cross-field rules
// (e.g. confirmPassword) can capture a lazy getter without a chicken-and-egg problem.
export function useForm<T extends FieldsConfig>(configFactory: (data: FormData<T>) => T) {
  const formData = reactive({}) as FormData<T>
  const errors = reactive({}) as FormErrors<T>

  const config = configFactory(formData)

  for (const key in config) {
    ;(formData as Record<string, string>)[key] = ''
    ;(errors as Record<string, string>)[key] = ''
  }

  function validateField(key: keyof T) {
    const rules = config[key]?.rules ?? []
    for (const rule of rules) {
      const value = (formData as Record<string, string>)[key as string] ?? ''
      const result = rule(value)
      if (result !== true) {
        ;(errors as Record<string, string>)[key as string] = result
        return
      }
    }
    ;(errors as Record<string, string>)[key as string] = ''
  }

  function validate(): boolean {
    for (const key in config) {
      validateField(key)
    }
    return Object.values(errors).every((e) => e === '')
  }

  const isValid = computed(() => Object.values(errors).every((e) => e === ''))

  return { formData, errors, validate, validateField, isValid }
}
`
