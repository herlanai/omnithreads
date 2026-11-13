"""Build the branded Finder installer, including Retina background artwork."""
import argparse
from pathlib import Path

import dmgbuild
from PIL import Image, ImageDraw, ImageFont

ROOT = Path(__file__).resolve().parent.parent
parser = argparse.ArgumentParser()
parser.add_argument("--arch", choices=("arm64", "amd64"), default="arm64")
ARCH = parser.parse_args().arch
APP = ROOT / "build" / "OmniThreads.app" if ARCH == "arm64" else ROOT / "build" / "intel" / "OmniThreads.app"
ART = ROOT / "build" / ("installer" if ARCH == "arm64" else "installer-intel")
ART.mkdir(parents=True, exist_ok=True)


def background(scale):
    canvas = Image.new("RGB", (720 * scale, 460 * scale), "#fafafa")
    draw = ImageDraw.Draw(canvas)

    def text(x, y, value, size, color="#101010", bold=False):
        font = ImageFont.truetype(
            "/System/Library/Fonts/Supplemental/"
            + ("Arial Bold.ttf" if bold else "Arial Unicode.ttf"), size * scale
        )
        draw.text((x * scale, y * scale), value, fill=color, font=font)

    logo = Image.open(ROOT / "assets" / "app-icon.png").convert("RGBA")
    logo = logo.resize((60 * scale, 60 * scale), Image.Resampling.LANCZOS)
    canvas.paste(logo, (44 * scale, 36 * scale), logo)
    text(116, 43, "OmniThreads", 28, bold=True)
    text(117, 80, "Where ideas meet and collaboration takes shape.", 13, "#737373")
    draw.line((48 * scale, 122 * scale, 672 * scale, 122 * scale), fill="#e5e5e5", width=scale)
    for x in (124, 412):
        draw.rounded_rectangle(
            (x * scale, 164 * scale, (x + 184) * scale, 332 * scale),
            radius=20 * scale, fill="#ffffff", outline="#e5e5e5", width=scale,
        )
    # Actual app and Applications icons are supplied by Finder, not painted here.
    draw.line((338 * scale, 240 * scale, 382 * scale, 240 * scale), fill="#888888", width=2 * scale)
    draw.line([(373 * scale, 231 * scale), (382 * scale, 240 * scale), (373 * scale, 249 * scale)], fill="#888888", width=2 * scale)
    text(205, 355, "Drag OmniThreads to Applications to install.", 15, "#464646")
    text(48, 422, "Built for people and agents to create together.", 11, "#888888")
    text(545, 422, "macOS · Apple Silicon" if ARCH == "arm64" else "macOS · Intel", 11, "#888888")
    path = ART / ("background.png" if scale == 1 else "background@2x.png")
    canvas.save(path)


for scale in (1, 2):
    background(scale)

dmgbuild.build_dmg(
    str(ROOT / "build" / f"OmniThreads-darwin-{ARCH}.dmg"),
    "OmniThreads",
    settings={
        "format": "UDZO",
        "files": [str(APP)],
        "symlinks": {"Applications": "/Applications"},
        "icon": str(ROOT / "assets" / "AppIcon.icns"),
        "background": str(ART / "background.png"),
        "window_rect": ((160, 120), (720, 460)),
        "icon_locations": {"OmniThreads.app": (216, 238), "Applications": (504, 238)},
        "icon_size": 96,
        "text_size": 13,
        "default_view": "icon-view",
        "show_toolbar": False,
        "show_status_bar": False,
        "show_pathbar": False,
        "show_sidebar": False,
        "include_icon_view_settings": True,
        "include_list_view_settings": False,
    },
)
