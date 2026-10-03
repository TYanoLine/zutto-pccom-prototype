import { createRoot } from 'react-dom/client';
import App from './App';
import HistoricalResearchPage from './admin/HistoricalResearchPage';

// The modem handshake is slightly quieter than line tones.
const sourceProto = AudioBufferSourceNode.prototype as any;
const originalConnect = sourceProto.connect;
sourceProto.connect = function (destination: AudioNode, ...rest: unknown[]) {
  if (destination instanceof AudioDestinationNode && this.buffer?.duration > 5) {
    const gain = destination.context.createGain();
    gain.gain.value = 0.71;
    originalConnect.call(this, gain);
    gain.connect(destination);
    return gain;
  }
  return originalConnect.call(this, destination, ...rest);
};

const path = window.location.pathname.replace(/\/+$/, '') || '/';
const Root = path === '/admin/research' ? HistoricalResearchPage : App;
createRoot(document.getElementById('root')!).render(<Root />);
