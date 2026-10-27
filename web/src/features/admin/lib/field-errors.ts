import { isApiError } from "@/lib/api-error";

/**
 * Maps the API's validation errors (field name -> tag) onto form fields.
 * Returns true when at least one field was set, so the caller can skip the toast.
 */
export function applyFieldErrors<Field extends string>(
  error: unknown,
  map: (fields: Record<string, string> | undefined) => { field: Field; message: string }[],
  setError: (field: Field, error: { message: string }) => void,
): boolean {
  if (!isApiError(error)) return false;
  const mapped = map(error.fields);
  for (const { field, message } of mapped) setError(field, { message });
  return mapped.length > 0;
}

/** Builds a mapper from a name table (API field -> form field). */
export function byFieldMap<Field extends string>(table: Record<string, Field>) {
  return (fields: Record<string, string> | undefined): { field: Field; message: string }[] =>
    Object.entries(fields ?? {}).flatMap(([key, message]) => {
      const field = table[key];
      return field ? [{ field, message }] : [];
    });
}
