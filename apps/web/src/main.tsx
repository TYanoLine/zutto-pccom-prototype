import { createRoot } from 'react-dom/client';
import App from './App';

// Keep the modem's monitor speaker a little below the telephone-line tones.
// Handshake PCM is always longer than 5 s; dial/ringback PCM is shorter.
const sourceProto = AudioBufferSourceNode.prototype as any;
const originalConnect = sourceProto.connect;
sourceProto.connect = function (destination: AudioNode, ...rest: unknown[]) {
  if (destination instanceof AudioDestinationNode && this.buffer?.duration > 5) {
    const gain = destination.context.createGain();
    gain.gain.value = 0.71; // about -3 dB
    originalConnect.call(this, gain);
    gain.connect(destination);
    return gain;
  }
  return originalConnect.call(this, destination, ...rest);
};

createRoot(document.getElementById('root')!).render(<App />);
