import { createRoot } from 'react-dom/client';
import App from './App';
import HistoricalResearchPage from './admin/HistoricalResearchPage';
import KnowledgeBbsPage from './poc/KnowledgeBbsPage';
import ImageArtifactPocPage from './poc/ImageArtifactPocPage';
import MaterializationLabViewerPage from './poc/MaterializationLabViewerPage';

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

function Home() {
  return <>
    <App />
    <a href="/poc/image-artifact" style={{position:'fixed',right:12,bottom:12,zIndex:50,padding:'7px 10px',fontFamily:'monospace',fontSize:12,color:'#9fffc0',background:'#07130dee',border:'1px solid #397a53',textDecoration:'none'}}>IMAGE FILE PoC</a>
  </>;
}

const path = window.location.pathname.replace(/\/+$/, '') || '/';
// PoC routes deliberately stay outside the historical host-program runtime.
const Root = path === '/admin/research'
  ? HistoricalResearchPage
  : path === '/poc/knowledge-bbs'
    ? KnowledgeBbsPage
    : path === '/poc/image-artifact'
      ? ImageArtifactPocPage
      : path === '/poc/materialization-lab-viewer'
        ? MaterializationLabViewerPage
        : Home;

createRoot(document.getElementById('root')!).render(<Root />);
