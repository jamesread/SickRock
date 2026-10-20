"""Signs in as the demo admin (password set by seed.sh) and pins the picocrank theme before capture.

repo-helper reuses one browser session for the whole INI, so localStorage
(the session token and theme) carries over between sections; each section
pins its own theme via light.py or dark.py.
"""
import time
from urllib.parse import parse_qs, urlparse

LOGIN_JS = """
const done = arguments[arguments.length - 1];
fetch('/api/sickrock.SickRock/Login', {
  method: 'POST',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({username: 'admin', password: 'fernbrook-demo'}),
}).then((r) => r.json()).then((r) => {
  if (r.token) localStorage.setItem('session-token', r.token);
  done(Boolean(r.token));
}).catch(() => done(false));
"""


def prepare_session(driver, theme):
    changed = False
    if not driver.execute_script("return localStorage.getItem('session-token');"):
        if not driver.execute_async_script(LOGIN_JS):
            raise RuntimeError("demo login failed")
        changed = True
    if driver.execute_script("return localStorage.getItem('picocrank-theme');") != theme:
        driver.execute_script("localStorage.setItem('picocrank-theme', arguments[0]);", theme)
        changed = True
    if not changed:
        return

    # An unauthenticated load was redirected to /login?redirect=<target>.
    url = urlparse(driver.current_url)
    target = parse_qs(url.query).get("redirect", [None])[0]
    if url.path == "/login" and target:
        driver.get("%s://%s%s" % (url.scheme, url.netloc, target))
    else:
        driver.refresh()
    time.sleep(3)
