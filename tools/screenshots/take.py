"""Take website screenshots of the real UI using sample data.

Builds nothing: run `npm run build` in frontend first. Needs Microsoft Edge.
Usage: python tools/screenshots/take.py [output-dir]   (default: docs/screens)
"""
import functools, http.server, os, shutil, subprocess, sys, tempfile, threading

ROOT = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
OUT = os.path.abspath(sys.argv[1]) if len(sys.argv) > 1 else os.path.join(ROOT, "docs", "screens")
EDGE = next(p for p in (
    r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe",
    r"C:\Program Files\Microsoft\Edge\Application\msedge.exe") if os.path.exists(p))

# (file name, query string)
SHOTS = [
    ("overview", "view=overview"),
    ("cleanup", "view=cleanup"),
    ("storage", "view=storage&click=%23sg-scan"),
    ("startup", "view=startup"),
    ("apps", "view=apps"),
    ("move", "view=move"),
    ("drivers", "view=drivers"),
]

work = tempfile.mkdtemp(prefix="shots-")
shutil.copytree(os.path.join(ROOT, "frontend", "dist"), work, dirs_exist_ok=True)
shutil.copy(os.path.join(os.path.dirname(__file__), "mock.js"), os.path.join(work, "mock.js"))
index = os.path.join(work, "index.html")
html = open(index, encoding="utf-8").read().replace("<head>", '<head><script src="/mock.js"></script>', 1)
open(index, "w", encoding="utf-8").write(html)

handler = functools.partial(http.server.SimpleHTTPRequestHandler, directory=work)
handler.log_message = lambda *a, **k: None
server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), handler)
port = server.server_address[1]
threading.Thread(target=server.serve_forever, daemon=True).start()

os.makedirs(OUT, exist_ok=True)
for name, query in SHOTS:
    dest = os.path.join(OUT, name + ".png")
    subprocess.run([EDGE, "--headless", "--disable-gpu", "--hide-scrollbars", "--virtual-time-budget=9000",
                    "--force-device-scale-factor=1", "--window-size=1200,872", "--force-prefers-reduced-motion", f"--screenshot={dest}",
                    f"http://127.0.0.1:{port}/?{query}"], check=True, capture_output=True)
    # The window has ~92 px of browser chrome that is not part of the page: crop it off.
    from PIL import Image
    img = Image.open(dest)
    img.crop((0, 0, img.width, img.height - 92)).save(dest, optimize=True)
    print(name, os.path.getsize(dest) // 1024, "KB")
server.shutdown()
shutil.rmtree(work, ignore_errors=True)
