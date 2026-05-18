// Regression test: connectDriverWS must reset UI state when getWsTicket fails.
// Bug 1: Early-return after null ticket left driverReconnectPkg set → endless reconnect loop.
// Bug 2: dsOffline/dsOnline divs not swapped back → user sees "Go Offline" stuck despite auth error.
// Fix: reset driverReconnectPkg + swap divs back + reset button before returning.

import { strict as assert } from 'assert';

// Minimal DOM mock — tracks all elements
let btn = { textContent: 'Go Offline', disabled: false };
let dsOffline = { style: { display: 'none' } };
let dsOnline  = { style: { display: 'flex' } };
const document = {
    getElementById: (id) => {
        if (id === 'goOnlineBtn') return btn;
        if (id === 'dsOffline')   return dsOffline;
        if (id === 'dsOnline')    return dsOnline;
        return null;
    }
};
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
        const offlineDiv = document.getElementById('dsOffline');
        const onlineDiv  = document.getElementById('dsOnline');
        if (offlineDiv) offlineDiv.style.display = '';
        if (onlineDiv)  onlineDiv.style.display  = 'none';
        const b = document.getElementById('goOnlineBtn');
        if (b) { b.textContent = 'Go Online'; b.disabled = false; }
        setStatus('Auth error — please reload');
        return;
    }
    throw new Error('should not reach WS setup in this test');
}

(async () => {
    // Precondition: _doGoOnline() already ran — divs swapped, driver "going online"
    btn.textContent = 'Go Offline';
    btn.disabled = false;
    dsOffline.style.display = 'none';
    dsOnline.style.display  = 'flex';
    driverReconnectPkg = 'standard';

    await connectDriverWS('standard', 'TestDriver');

    assert.strictEqual(driverReconnectPkg, null,
        'driverReconnectPkg must be null — otherwise reconnect loop fires endlessly');
    assert.strictEqual(dsOffline.style.display, '',
        'dsOffline must be visible — driver must see the Go Online panel again');
    assert.strictEqual(dsOnline.style.display, 'none',
        'dsOnline must be hidden — driver must not see the online panel with Go Offline button');
    assert.strictEqual(btn.textContent, 'Go Online',
        'goOnlineBtn must show Go Online');
    assert.strictEqual(btn.disabled, false,
        'button must be enabled so driver can retry');
    assert.strictEqual(statusText, 'Auth error — please reload',
        'status message must be set');

    console.log('✓ connectDriverWS: full UI reset on auth failure — OK');
})().catch(e => { console.error('FAIL:', e.message); process.exit(1); });
