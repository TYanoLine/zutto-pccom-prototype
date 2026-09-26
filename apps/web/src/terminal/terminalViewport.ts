export function terminalViewportHeight(
  canvasHeight: number,
  viewportTop: number,
  visibleBottom: number,
) {
  const available = Math.floor(visibleBottom - viewportTop - 4);
  return Math.min(canvasHeight, Math.max(96, available));
}

export function terminalCursorTargetScrollTop(
  canvasHeight: number,
  viewportHeight: number,
  cursorTop: number,
  rowHeight: number,
) {
  const maxScrollTop = Math.max(0, canvasHeight - viewportHeight);
  const target = cursorTop - viewportHeight + rowHeight * 2;
  return Math.max(0, Math.min(maxScrollTop, target));
}
