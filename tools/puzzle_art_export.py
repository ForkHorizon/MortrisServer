#!/usr/bin/env python3
"""Build the assembled-house art payload from the Puzzle Unity assets.

Produces a directory of cropped WebP images plus a manifest, for
`analytics-server import-puzzle-art`.

    python3 tools/puzzle_art_export.py \\
        --catalog  <Puzzle>/Assets/StreamingAssets/PuzzleAnalytics/puzzle_gravity_test.catalog.json \\
        --project-root <Puzzle> \\
        --out-dir ./house-art

Each image is cropped to its opaque bounding box. That crop's extent is
exactly the union of the house's block bounds, so the dashboard can place
it with the rect it already computes and no offset is stored anywhere —
verified by overlaying block outlines on the art.

The source is `composite.png`, NOT the catalogue's `preview_asset_key`:
that points at `static_packed.png`, which is the window/lights layer, not
the assembled house.
"""

import argparse
import json
import os
import sys

from PIL import Image

# 900px on the long edge keeps a house readable at full width on a desktop
# while holding the whole set around 4 MB. Quality 82 is where WebP stops
# visibly softening the brick edges in these assets.
MAX_EDGE = 900
WEBP_QUALITY = 82


def house_art_source(house, project_root):
    """composite.png next to the house's other art, or None."""
    preview = house.get("preview_asset_key") or ""
    if not preview:
        return None
    candidate = os.path.join(project_root, os.path.dirname(preview), "composite.png")
    return candidate if os.path.exists(candidate) else None


def render(source, destination):
    """Crop to opaque pixels, downscale, write WebP. Returns (w, h)."""
    with Image.open(source) as image:
        image = image.convert("RGBA")
        box = image.getbbox()
        if box is None:
            return None
        cropped = image.crop(box)
    cropped.thumbnail((MAX_EDGE, MAX_EDGE))
    cropped.save(destination, "WEBP", quality=WEBP_QUALITY, method=4)
    return cropped.size


def build(catalog, project_root, out_dir, report):
    entries = []
    for city in catalog["cities"]:
        for house in city["houses"]:
            report["houses"] += 1
            source = house_art_source(house, project_root)
            if source is None:
                report["without_art"] += 1
                continue
            name = f"{city['city_id']}_{house['house_id']}.webp"
            size = render(source, os.path.join(out_dir, name))
            if size is None:
                report["fully_transparent"] += 1
                continue
            entries.append({
                "city_id": city["city_id"], "house_id": house["house_id"],
                "file": name, "media_type": "image/webp",
                "width": size[0], "height": size[1],
            })
    return {
        "schema_version": 1,
        "content_revision": catalog["content_revision"],
        "houses": entries,
    }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--catalog", required=True)
    parser.add_argument("--project-root", required=True)
    parser.add_argument("--out-dir", required=True)
    args = parser.parse_args()

    os.makedirs(args.out_dir, exist_ok=True)
    with open(args.catalog, encoding="utf8") as handle:
        catalog = json.load(handle)
    report = dict(houses=0, without_art=0, fully_transparent=0)
    manifest = build(catalog, args.project_root, args.out_dir, report)
    manifest_path = os.path.join(args.out_dir, "manifest.json")
    with open(manifest_path, "w", encoding="utf8") as handle:
        json.dump(manifest, handle, indent=1)

    total = sum(os.path.getsize(os.path.join(args.out_dir, h["file"])) for h in manifest["houses"])
    print(f"revision {manifest['content_revision'][:16]}…")
    print(f"{len(manifest['houses'])} of {report['houses']} houses rendered, {total / 1_000_000:.1f} MB total")
    # Missing art is reported rather than swallowed: a house without it
    # renders as a bare diagram, and that should be visible here instead
    # of being discovered as a blank panel in the dashboard.
    for key in ("without_art", "fully_transparent"):
        if report[key]:
            print(f"skipped — {key.replace('_', ' ')}: {report[key]}", file=sys.stderr)
    print(f"wrote {manifest_path}")


if __name__ == "__main__":
    main()
