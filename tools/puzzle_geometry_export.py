#!/usr/bin/env python3
"""Build a Mortris block-geometry payload from the Puzzle Unity assets.

The catalogue that Puzzle's exporter uploads describes where each block
sits but not what shape it is, so the dashboard can plot points and not
draw a house. This fills that in for a revision that already exists on
the server, without changing its content_revision — see
internal/analytics/puzzle_geometry.go for why shapes attach to a
revision rather than living inside it.

    python3 tools/puzzle_geometry_export.py \\
        --catalog  <Puzzle>/Assets/StreamingAssets/PuzzleAnalytics/puzzle_gravity_test.catalog.json \\
        --project-root <Puzzle> \\
        --out geometry.json

POST the result to
/api/v1/projects/{projectID}/puzzle-content/{revision}/geometry.
"""

import argparse
import json
import os
import re
import sys

from PIL import Image

# Unity places a sprite by its pivot, which is centred on the FULL
# texture rect including transparent padding. Bounds are therefore
# derived from the full rect first and only then trimmed to the visible
# pixels, or every trimmed block would drift by half its padding.
DEFAULT_PPU = 100.0
MILLI = 1000.0

# ponytail: column silhouette, not a true contour trace. Each sampled
# column contributes its topmost and bottommost solid pixel, so the
# result is exact for the vertically simple shapes puzzle pieces
# overwhelmingly are, and degrades to something bounds-like on a piece
# with holes or deep overhangs. Upgrade to marching squares only if a
# real house visibly reads wrong.
OUTLINE_COLUMNS = 16
ALPHA_THRESHOLD = 8


def quantize(value):
    return int(round(value * MILLI))


def sprite_ppu(png_path):
    """Read spritePixelsToUnits from the sibling .meta, else the default."""
    meta = png_path + ".meta"
    if not os.path.exists(meta):
        return DEFAULT_PPU
    with open(meta, encoding="utf8", errors="ignore") as handle:
        found = re.search(r"spritePixelsToUnits:\s*([\d.]+)", handle.read())
    return float(found.group(1)) if found and float(found.group(1)) > 0 else DEFAULT_PPU


def alpha_columns(alpha, width, height):
    """Topmost and bottommost solid pixel per sampled column, in pixels."""
    pixels = alpha.load()
    step = max(1, width // OUTLINE_COLUMNS)
    columns = []
    for x in range(0, width, step):
        top, bottom = None, None
        for y in range(height):
            if pixels[x, y] >= ALPHA_THRESHOLD:
                if top is None:
                    top = y
                bottom = y
        if top is not None:
            columns.append((x, top, bottom))
    return columns


def block_geometry(png_path, center_x, center_y):
    """Bounds and outline for one block, in world milli-units."""
    with Image.open(png_path) as image:
        image = image.convert("RGBA")
        width, height = image.size
        alpha = image.getchannel("A")
        visible = alpha.getbbox()
        columns = alpha_columns(alpha, width, height)
    if visible is None:
        return None
    ppu = sprite_ppu(png_path)
    # Pixel space is y-down with its origin top-left; world space is
    # y-up centred on the pivot. This maps one to the other.
    def to_world(px, py):
        return (center_x + (px - width / 2.0) / ppu,
                center_y + (height / 2.0 - py) / ppu)

    left, top, right, bottom = visible
    min_x, max_y = to_world(left, top)
    max_x, min_y = to_world(right, bottom)
    outline = ([to_world(x, t) for x, t, _ in columns] +
               [to_world(x, b + 1) for x, _, b in reversed(columns)])
    return {
        "bounds_milli": {
            "min_x": quantize(min_x), "min_y": quantize(min_y),
            "max_x": quantize(max_x), "max_y": quantize(max_y),
        },
        "outline_milli": [[quantize(x), quantize(y)] for x, y in outline],
    }


def clamp_outline(geometry):
    """Keep every outline point inside bounds — the server rejects strays.

    Rounding at the edges can push a point a single milli-unit past the
    bound it was derived from; clamping is correct rather than lenient,
    since the point genuinely lies on the boundary.
    """
    bounds = geometry["bounds_milli"]
    clamped = []
    for x, y in geometry["outline_milli"]:
        clamped.append([
            min(max(x, bounds["min_x"]), bounds["max_x"]),
            min(max(y, bounds["min_y"]), bounds["max_y"]),
        ])
    geometry["outline_milli"] = clamped
    return geometry


def pieces_dir(house, project_root):
    preview = house.get("preview_asset_key") or ""
    if not preview:
        return None
    return os.path.join(project_root, os.path.dirname(preview), "pieces")


def house_blocks(house, city_id, project_root, report):
    directory = pieces_dir(house, project_root)
    if not directory or not os.path.isdir(directory):
        report["houses_without_pieces"] += 1
        return []
    rows = []
    for block in house["blocks"]:
        png = os.path.join(directory, (block.get("visual_key") or "") + ".png")
        if not os.path.exists(png):
            report["blocks_without_sprite"] += 1
            continue
        geometry = block_geometry(png, block["local_x_milli"] / MILLI,
                                  block["local_y_milli"] / MILLI)
        if geometry is None:
            report["blocks_fully_transparent"] += 1
            continue
        rows.append(dict(city_id=city_id, house_id=house["house_id"],
                         block_id=block["block_id"], **clamp_outline(geometry)))
    return rows


def build(catalog, project_root, report):
    blocks = []
    for city in catalog["cities"]:
        for house in city["houses"]:
            report["houses"] += 1
            blocks.extend(house_blocks(house, city["city_id"], project_root, report))
    return {
        "schema_version": 1,
        "content_revision": catalog["content_revision"],
        "blocks": blocks,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--catalog", required=True)
    parser.add_argument("--project-root", required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args()

    with open(args.catalog, encoding="utf8") as handle:
        catalog = json.load(handle)
    report = dict(houses=0, houses_without_pieces=0,
                  blocks_without_sprite=0, blocks_fully_transparent=0)
    payload = build(catalog, args.project_root, report)
    with open(args.out, "w", encoding="utf8") as handle:
        json.dump(payload, handle, separators=(",", ":"))

    print(f"revision {payload['content_revision'][:16]}…")
    print(f"{len(payload['blocks'])} blocks with geometry, {report['houses']} houses")
    # Skipped blocks are reported rather than swallowed: a house missing
    # its pieces directory renders shapeless, and that must be visible
    # here instead of being discovered as a blank house in the dashboard.
    for key in ("houses_without_pieces", "blocks_without_sprite",
                "blocks_fully_transparent"):
        if report[key]:
            print(f"skipped — {key.replace('_', ' ')}: {report[key]}", file=sys.stderr)
    print(f"wrote {args.out} ({os.path.getsize(args.out) / 1_000_000:.1f} MB)")


if __name__ == "__main__":
    main()
