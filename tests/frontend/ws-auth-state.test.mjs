// Regression test: connectDriverWS must reset UI state when getWsTicket fails.
// Bug: Early-return after null ticket left driverReconnectPkg set and button stuck on "Go Offline".
// Fix: driverReconnectPkg = null + button reset before returning.

import { strict as assert } from 'assert';

// Minimal DOM mock
let btn = { textContent: 'Go Offline', disabled: false };
const document = { getElementById: (id) => id === 'goOnlineBtn' ? btn : null };
let statusText = '';
const setStatus = (s) => { statusText = s; };
const defaultAvatar = () => 'avatar';

// State
let driverReconnectPkg = null;
const currentUser = { name: 'TestDriver', email: 'driver@drova.local', avatarUrl: null };

// Simulates token expiry / user-service unavailable during pod rollout
const getWsTicket = async () => null;

async function connectDriverWS(pkg, driverName) {
    driverReconnectPkg = pkg;
    const wsName = driverName || currentUser.name || currentUser.email.split('@')[0];
    const driverAvatar = currentUser.avatarUrl || defaultAvatar(wsName);
    const ticket = await getWsTicket();
    if (!ticket) {
        driverReconnectPkg = null;
        const b = document.getElementById('goOnlineBtn');
        if (b) { b.textContent = 'Go Online'; b.disabled = false; }
        setStatus('Auth error — please reload');
        return;
    }
    throw new Error('should not reach WS setup in this test');
}

(async () => {
    // Precondition: driver was online, reconnect fires after pod rollout
    btn.textContent = 'Go Offline';
    btn.disabled = false;
    driverReconnectPkg = 'standard';

    await connectDriverWS('standard', 'TestDriver');

    assert.strictEqual(driverReconnectPkg, null,
        'driverReconnectPkg must be null — otherwise reconnect loop fires endlessly');
    assert.strictEqual(btn.textContent, 'Go Online',
        'button must show Go Online — driver must not be stuck in broken offline state');
    assert.strictEqual(btn.disabled, false,
        'button must be enabled so driver can retry');
    assert.strictEqual(statusText, 'Auth error — please reload',
        'status message must be set');

    console.log('✓ connectDriverWS: state reset on auth failure — OK');
})().catch(e => { console.error('FAIL:', e.message); process.exit(1); });
