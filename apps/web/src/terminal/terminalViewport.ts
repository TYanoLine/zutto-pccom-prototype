export function terminalViewportHeight(
  canvasHeight: number,
  viewportTop: number,
  visibleBottom: number,
  trailingUiHeight: number,
) {
  const available = Math.floor(visibleBottom - viewportTop - trailingUiHeight - 4);
  return Math.min(canvasHeight, Math.max(96, available));
}

export function terminalCursorScrollTop(
  currentScrollTop: number,
  viewportHeight: number,
  cursorTop: number,
  rowHeight: number,
) {
  const padding = rowHeight;
  if (cursorTop - currentScrollTop < padding) {
    return Math.max(0, cursorTop - padding);
  }

  const cursorBottom = cursorTop + rowHeight;
  if (cursorBottom - currentScrollTop > viewportHeight - padding) {
    return Math.max(0, cursorBottom - viewportHeight + padding);
  }

  return currentScrollTop;
}
