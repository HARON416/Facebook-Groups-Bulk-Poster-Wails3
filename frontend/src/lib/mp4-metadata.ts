interface Box { type: string; start: number; end: number }
const text = (view: DataView, at: number) => String.fromCharCode(...new Uint8Array(view.buffer, view.byteOffset + at, 4));
const invalid = () => new Error('invalid-video');

function boxes(view: DataView, start = 0, end = view.byteLength): Box[] {
  const result: Box[] = [];
  for (let offset = start; offset < end;) {
    if (end - offset < 8) throw invalid();
    let size = view.getUint32(offset), header = 8;
    if (size === 1) {
      if (end - offset < 16) throw invalid();
      size = Number(view.getBigUint64(offset + 8)); header = 16;
    } else if (size === 0) size = end - offset;
    if (!Number.isSafeInteger(size) || size < header || offset + size > end) throw invalid();
    result.push({ type: text(view, offset + 4), start: offset + header, end: offset + size });
    offset += size;
  }
  return result;
}

// Inspect container metadata rather than asking the webview to decode H.264.
// Skip media payloads and only read the movie metadata (including moov-at-end).
export async function mp4Metadata(file: Blob, signal: AbortSignal): Promise<{ duration: number; width: number; height: number }> {
  const check = () => { if (signal.aborted) throw new DOMException('Aborted', 'AbortError'); };
  for (let offset = 0; offset < file.size;) {
    check();
    const header = new DataView(await file.slice(offset, offset + 16).arrayBuffer());
    check();
    if (header.byteLength < 8) throw invalid();
    let size = header.getUint32(0), length = 8;
    if (size === 1) {
      if (header.byteLength < 16) throw invalid();
      size = Number(header.getBigUint64(8)); length = 16;
    } else if (size === 0) size = file.size - offset;
    if (!Number.isSafeInteger(size) || size < length || offset + size > file.size) throw invalid();
    if (text(header, 4) === 'moov') {
      // Avoid allocating an arbitrarily large malformed metadata block.
      if (size > 32 * 1024 * 1024) throw invalid();
      const view = new DataView(await file.slice(offset + length, offset + size).arrayBuffer());
      check();
      const children = boxes(view);
      const mvhd = children.find(box => box.type === 'mvhd');
      if (!mvhd || mvhd.end - mvhd.start < 20) throw invalid();
      const version = view.getUint8(mvhd.start);
      if (version !== 0 && version !== 1) throw invalid();
      if (version === 1 && mvhd.end - mvhd.start < 32) throw invalid();
      const scale = view.getUint32(mvhd.start + (version === 1 ? 20 : 12));
      const ticks = version === 1 ? Number(view.getBigUint64(mvhd.start + 24)) : view.getUint32(mvhd.start + 16);
      const duration = ticks / scale;
      if (!scale || !Number.isFinite(duration) || duration <= 0) throw invalid();
      for (const track of children.filter(box => box.type === 'trak')) {
        const parts = boxes(view, track.start, track.end);
        const tkhd = parts.find(box => box.type === 'tkhd'), mdia = parts.find(box => box.type === 'mdia');
        if (!tkhd || !mdia || tkhd.end - tkhd.start < 84) continue;
        const handler = boxes(view, mdia.start, mdia.end).find(box => box.type === 'hdlr');
        if (!handler || handler.end - handler.start < 12 || text(view, handler.start + 8) !== 'vide') continue;
        const width = view.getUint32(tkhd.end - 8) / 65536, height = view.getUint32(tkhd.end - 4) / 65536;
        if (width > 0 && height > 0) return { duration, width, height };
      }
      throw invalid();
    }
    offset += size;
  }
  throw invalid();
}
