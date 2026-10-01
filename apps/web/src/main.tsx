import { createRoot } from 'react-dom/client';
import App from './App';
import HistoricalResearchPage from './admin/HistoricalResearchPage';
import KnowledgeBbsPage from './poc/KnowledgeBbsPage';
import ImageArtifactPocPage from './poc/ImageArtifactPocPage';
import PersonaLabPage from './poc/PersonaLabPage';
import PersonaTimelinePocPage from './poc/PersonaTimelinePocPage';

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
  const devLinkStyle = {padding:'7px 10px',fontFamily:'monospace',fontSize:12,color:'#9fffc0',background:'#07130dee',border:'1px solid #397a53',textDecoration:'none'} as const;
  return <>
    <App />
    <div className="dev-shortcuts" style={{position:'fixed',right:12,bottom:12,zIndex:50,display:'flex',gap:8,flexWrap:'wrap',justifyContent:'flex-end'}}>
      <a href="/poc/persona-timeline" style={devLinkStyle}>PERSONA TIME</a>
      <a href="/poc/persona-lab" style={devLinkStyle}>PERSONA LAB</a>
      <a href="/poc/image-artifact" style={devLinkStyle}>IMAGE FILE PoC</a>
    </div>
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
      : path === '/poc/persona-lab'
        ? PersonaLabPage
        : Home;

createRoot(document.getElementById('root')!).render(<Root />);