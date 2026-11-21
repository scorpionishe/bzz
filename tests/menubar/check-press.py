"""Record WindowServer mouse events at 60 fps and inspect every Bzz background frame.

Requires a running signed /Applications/Bzz.app, an unlocked 2x display,
Accessibility/Screen Recording, ffmpeg, Swift, agent-desktop and numpy.
"""
import argparse
import csv
import json
from pathlib import Path
import subprocess
import time

import numpy as np

HERE = Path(__file__).resolve().parent


def run(*args):
    return subprocess.check_output([str(a) for a in args])


def record(directory, drag=False):
    directory.mkdir(parents=True, exist_ok=True)
    probe = directory / "Probe"
    subprocess.run(["xcrun", "swiftc", HERE / "Probe.swift", "-o", probe], check=True)

    def read(command):
        return json.loads(run(probe, command))

    def wait_menu(expected):
        deadline = time.monotonic() + 3
        while time.monotonic() < deadline:
            if read("menu")["open"] == expected:
                return
            time.sleep(.05)
        raise AssertionError(f"Expected menu open={expected}")

    if read("menu")["open"]:
        raise RuntimeError("Close Bzz's menu before the recording")
    item = read("item")
    if item["scale"] != 2:
        raise RuntimeError("This pixel check requires a 2x display")
    width = int(item["screenWidth"] * 2)
    events = []
    clip = directory / "press.mov"
    with (directory / "capture.log").open("w") as log:
        recorder = subprocess.Popen([
            "ffmpeg", "-nostdin", "-y", "-v", "warning", "-f", "avfoundation",
            "-capture_cursor", "0", "-framerate", "60", "-i", "Capture screen 0:none",
            "-vf", f"crop={width}:100:0:0", "-t", "17", "-c:v", "png",
            "-stats_period", "0.1", "-progress", "pipe:1", clip,
        ], stdout=subprocess.PIPE, stderr=log, text=True)
        try:
            for line in recorder.stdout:
                if line.startswith("out_time_us=") and line.split("=")[1].strip().isdigit():
                    pts = int(line.split("=")[1]) / 1_000_000
                    if pts >= 2:
                        epoch = time.monotonic() - pts
                        break
            else:
                raise RuntimeError("Recording did not start")

            def action(command, opened):
                before = time.monotonic()
                result = json.loads(run(probe, command))
                wait_menu(opened)
                events.append({"action": command, "start": before - epoch,
                               "end": time.monotonic() - epoch, "menuOpen": opened,
                               **{k: v - epoch if k in ("down", "up") else v for k, v in result.items()}})
                time.sleep(.25)

            action("hold", True)
            action("click", False)
            action("hold", True)
            before = time.monotonic()
            run("agent-desktop", "--headed", "press", "escape")
            wait_menu(False)
            events.append({"action": "escape", "start": before - epoch,
                           "end": time.monotonic() - epoch, "menuOpen": False})
            time.sleep(.25)
            action("hold", True)
            action("click", False)
            if drag:
                before = time.monotonic()
                result = json.loads(run(probe, "drag", "-70"))
                wait_menu(False)
                events.append({"action": "commandDrag", "start": before - epoch,
                               "end": time.monotonic() - epoch, **result})
                # Exercise the newly positioned pointer area after the native drag.
                action("hold", True)
                action("click", False)
            recorder.communicate(timeout=20)
            if recorder.returncode:
                raise RuntimeError("Screen capture failed")
        finally:
            if recorder.poll() is None:
                recorder.terminate()
                recorder.wait(timeout=5)
            if read("menu")["open"]:
                run("agent-desktop", "--headed", "press", "escape")

    metadata = {"macOS": run("sw_vers").decode(), "item": item, "width": width,
                "events": events, "timingUncertaintySeconds": .12}
    (directory / "events.json").write_text(json.dumps(metadata, indent=2) + "\n")
    return analyze(directory)


def analyze(directory):
    meta = json.loads((directory / "events.json").read_text())
    clip = directory / "press.mov"
    width = meta["width"]
    details = json.loads(run("ffprobe", "-v", "error", "-select_streams", "v:0",
                            "-show_frames", "-show_entries", "frame=best_effort_timestamp_time",
                            "-of", "json", clip))
    timestamps = [float(f["best_effort_timestamp_time"]) for f in details["frames"]]
    nominal = json.loads(run("ffprobe", "-v", "error", "-select_streams", "v:0",
                             "-show_entries", "stream=r_frame_rate,avg_frame_rate", "-of", "json", clip))["streams"][0]
    if nominal["r_frame_rate"] != "60/1":
        raise RuntimeError(f"Expected 60 fps, got {nominal}")
    first = meta["events"][0]
    template_time = max(.5, first["down"] - .1)
    reference = np.frombuffer(run("ffmpeg", "-v", "error", "-ss", str(template_time), "-i", clip,
                                  "-frames:v", "1", "-pix_fmt", "rgb24", "-f", "rawvideo", "pipe:1"),
                              np.uint8).reshape(100, width, 3)
    center = round(first["x"] * 2)
    # Opaque pixels inside the Union Jack, independent of the system background.
    dx = np.array([-12, -8, -4, 0, 4, 8, 12] * 3)
    dy = np.repeat([26, 34, 42], 7)
    template = reference[dy, center + dx].astype(np.int16)
    candidates = np.arange(max(60, center - 650), min(width - 60, center + 650))
    decoder = subprocess.Popen(["ffmpeg", "-v", "error", "-i", clip, "-fps_mode", "passthrough",
                                "-pix_fmt", "rgb24", "-f", "rawvideo", "pipe:1"], stdout=subprocess.PIPE)
    rows = []
    for index, pts in enumerate(timestamps):
        data = decoder.stdout.read(width * 100 * 3)
        if len(data) != width * 100 * 3:
            raise RuntimeError("Incomplete decoded frame")
        frame = np.frombuffer(data, np.uint8).reshape(100, width, 3)
        scores = np.abs(frame[dy[:, None], candidates[None, :] + dx[:, None]].astype(np.int16)
                        - template[:, None, :]).mean(axis=(0, 2))
        best = int(scores.argmin())
        x, error = int(candidates[best]), float(scores[best])
        drag = any(e["action"] == "commandDrag" and e["start"] - .12 <= pts <= e["end"] + .12
                   for e in meta["events"])

        def patch(offset):
            return float(np.median(frame[29:40, x + offset - 2:x + offset + 3, 1]))

        left, right = patch(-30) - patch(-48), patch(30) - patch(48)
        oval = left > 5 and right > 5
        if error > 15 and not drag:
            raise RuntimeError(f"Cannot identify Bzz in frame {index}: error={error}")
        rows.append({"frame": index, "pts": pts, "centerX": x, "templateError": round(error, 3),
                     "leftContrast": left, "rightContrast": right, "commandDrag": drag, "oval": oval})
    if decoder.wait() != 0:
        raise RuntimeError("Decoder failed")
    with (directory / "frames.csv").open("w") as output:
        writer = csv.DictWriter(output, fieldnames=rows[0].keys(), lineterminator="\n")
        writer.writeheader()
        writer.writerows(rows)
    ordinary = [r for r in rows if not r["commandDrag"]]
    result = {"frames": len(rows), "ordinaryFrames": len(ordinary),
              "ovalFrames": [r["frame"] for r in ordinary if r["oval"]],
              "fps": nominal, "events": meta["events"]}
    (directory / "result.json").write_text(json.dumps(result, indent=2) + "\n")
    # The shared evidence is centered on Bzz, not a neighbouring status item.
    run("ffmpeg", "-v", "error", "-y", "-i", clip, "-vf", f"crop=104:76:{center - 52}:0",
        "-c:v", "libx264", "-crf", "15", "-pix_fmt", "yuv420p", "-fps_mode", "passthrough", directory / "bzz-60fps.mp4")
    run("ffmpeg", "-v", "error", "-y", "-ss", str(first["down"] + .08), "-i", clip,
        "-vf", f"crop=104:76:{center - 52}:0,tile=6x4", "-frames:v", "1", directory / "hold-frames.png")
    if result["ovalFrames"]:
        raise AssertionError(f"Oval detected: {result['ovalFrames']}")
    print(json.dumps({k: v for k, v in result.items() if k != "events"}, indent=2))
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("directory", type=Path)
    parser.add_argument("--drag", action="store_true")
    parser.add_argument("--analyze", action="store_true")
    args = parser.parse_args()
    (analyze if args.analyze else lambda d: record(d, args.drag))(args.directory.resolve())
