// What the connected host lets this client offer. It comes from the server in
// `dial_result` / `resume_result` (`capabilities`), never from a phone number.
export type HostCapabilities = {
  generationTrace: boolean;
};

export function noCapabilities(): HostCapabilities {
  return { generationTrace: false };
}

// Anything that is not an object, or a flag that is not exactly `true`, is off.
// An older server that sends no `capabilities` therefore enables nothing.
export function parseHostCapabilities(value: unknown): HostCapabilities {
  if (!value || typeof value !== 'object') return noCapabilities();
  const raw = value as { generation_trace?: unknown };
  return { generationTrace: raw.generation_trace === true };
}
