import { createRoot } from 'react-dom/client';
import App from './App';

// Keep the modem's monitor speaker a little below the telephone-line tones.
// Web Audio has no per-AudioContext destination volume, so intercept only
// long PCM buffers (the synthesized modem handshake is always > 5 s) and
// insert a modest -3 dB gain stage. Telephone dial/ringback buffers are short
// and are left untouched.
const originalConnect = AudioBufferSourceNode.prototype.connect;
AudioBufferSourceNode.prototype.connect = function (...args: Parameters<AudioBufferSourceNode['connect']>) {
  const destination = args[0];
  if (destination instanceof AudioDestinationNode && this.buffer && this.buffer.duration > 5) {
    const gain = destination.context.createGain();
    gain.gain.value = 0.71;
    originalConnect.call(this, gain);
    gain.connect(destination);
    return gain as unknown as AudioNode;
  }
  return originalConnect.apply(this, args as never);
} as AudioBufferSourceNode['connect'];

createRoot(document.getElementById('root')!).render(<App />);
