import { useState } from 'react';
import type { DialMode } from '../audio/dialLineAudio';
import type { RegisteredCenter } from './CenterDirectory';

type Props = {
  centers: RegisteredCenter[];
  disabled?: boolean;
  onDial(center: RegisteredCenter): void;
  onAdd(center: Omit<RegisteredCenter, 'id' | 'builtIn'>): void;
  onDelete(id: string): void;
};

export function CenterDirectoryPanel({ centers, disabled = false, onDial, onAdd, onDelete }: Props) {
  const [name, setName] = useState('');
  const [phone, setPhone] = useState('');
  const [dialMode, setDialMode] = useState<DialMode>('tone');

  function addCenter() {
    const cleanPhone = phone.replace(/\D/g, '');
    const cleanName = name.trim();
    if (!cleanPhone || !cleanName) return;
    onAdd({ name: cleanName, phone: cleanPhone, dialMode });
    setName('');
    setPhone('');
    setDialMode('tone');
  }

  return (
    <details className="center-panel" open>
      <summary>CENTER DIRECTORY / センター登録</summary>
      <div className="center-list">
        {centers.map(center => (
          <div className="center-row" key={center.id}>
            <div className="center-main">
              <strong>{center.name}</strong>
              <span>{center.phone} / {center.dialMode.toUpperCase()}{center.builtIn ? ' / PRESET' : ''}</span>
            </div>
            <div className="center-actions">
              <button type="button" className="center-call" disabled={disabled} onClick={() => onDial(center)}>CALL</button>
              {!center.builtIn && <button type="button" className="center-delete" disabled={disabled} onClick={() => onDelete(center.id)}>DEL</button>}
            </div>
          </div>
        ))}
      </div>
      <div className="settings-group-title">NEW CENTER</div>
      <div className="center-add-grid">
        <label>NAME<input value={name} maxLength={64} onChange={e => setName(e.target.value)} placeholder="CENTER NAME" /></label>
        <label>PHONE<input value={phone} maxLength={24} onChange={e => setPhone(e.target.value)} placeholder="0451234567" /></label>
        <label>DIAL<select value={dialMode} onChange={e => setDialMode(e.target.value as DialMode)}><option value="tone">TONE / DTMF</option><option value="pulse">PULSE / 10pps</option></select></label>
        <button type="button" className="center-add" disabled={disabled || !name.trim() || !phone.replace(/\D/g, '')} onClick={addCenter}>REGISTER</button>
      </div>
      <div className="settings-footnote">CALL は登録番号へ ATDT / ATDP 相当で発信します。登録内容はこのブラウザに保存されます。</div>
    </details>
  );
}
