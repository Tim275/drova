# ZAP-Hook: injiziert "Authorization: Bearer <ZAP_AUTH_TOKEN>" in JEDEN Request,
# damit der Full/Active Scan auch die Wege HINTER dem Login prüft (authenticated).
# Der -z-Replacer scheitert am Leerzeichen in "Bearer <token>" (ZAP splittet auf
# Spaces) — die ZAP-API im Hook kann den Wert dagegen sauber setzen.
#
# Aktiviert via:  zap-full-scan.py --hook=/zap/wrk/auth_hook.py   (+ -e ZAP_AUTH_TOKEN)
import os


def zap_started(zap, target):
    token = os.environ.get("ZAP_AUTH_TOKEN", "")
    if not token:
        print("auth_hook: kein ZAP_AUTH_TOKEN — Scan laeuft unauthenticated")
        return
    zap.replacer.add_rule(
        description="auth-bearer",
        enabled=True,
        matchtype="REQ_HEADER",
        matchregex=False,
        matchstring="Authorization",
        replacement="Bearer " + token,
        initiators="",
    )
    print("auth_hook: Bearer-Token injiziert — authenticated scan aktiv")
