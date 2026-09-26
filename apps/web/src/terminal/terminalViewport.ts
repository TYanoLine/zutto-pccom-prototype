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

export function mobileTerminalRows(
  viewportWidth: number,
  availableHeight: number,
  columns = 80,
  cellWidth = 8,
  cellHeight = 16,
) {
  const logicalWidth = columns * cellWidth;
  if (viewportWidth <= 0 || availableHeight <= 0) return 25;
  const scale = viewportWidth / logicalWidth;
  const displayedRowHeight = cellHeight * scale;
  return Math.max(25, Math.min(120, Math.floor(availableHeight / displayedRowHeight)));
}
