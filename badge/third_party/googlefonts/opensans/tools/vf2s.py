#!/usr/bin/env python3
# -*- coding: utf-8 -*-

# ========================================================
# vf2s.py
# Copyright 2019 Google, LLC
# Apache License, v2.0
#
# A variable font to static font instance generator
# + unique name table writer
# =======================================================

# PyInstaller build for macOS architecture
#
# pyinstaller -c --onefile --hidden-import=fontTools --clean --distpath="dist/macos64" -n vf2s vf2s.py

import os
import sys
import argparse

from fontTools.ttLib import TTFont
from fontTools.varLib.mutator import instantiateVariableFont

SCRIPT_VERSION = "v0.6.0"

# Define font name for font path and name table re-writes

FONTNAME = "Open Sans"

# Default axis values (when not explicitly included on the command line)
DEFAULT_WEIGHT = 400
DEFAULT_WIDTH = 100

# Min/Max of design axis range values for validity checking command line entries
WEIGHT_MIN = 300
WEIGHT_MAX = 800

WIDTH_MIN = 60
WIDTH_MAX = 100

# macOS rendering bit
# used for workaround fix for fontTools varLib.mutator bug
MAC_OVERLAP_RENDERING_BIT = 1 << 6


def set_mac_overlap_rendering_bit(font):
    """Sets the bit6 macOS overlap rendering bit."""
    glyf = font["glyf"]
    for glyph_name in glyf.keys():
        glyph = glyf[glyph_name]
        # Only needs to be set for glyphs with contours
        if glyph.numberOfContours > 0:
            glyph.flags[0] |= MAC_OVERLAP_RENDERING_BIT
    return font


def main():
    parser = argparse.ArgumentParser(
        description="A variable font to static instance generator."
    )
    parser.add_argument(
        "--weight",
        default=DEFAULT_WEIGHT,
        type=int,
        help="Weight axis value ({}-{})".format(WEIGHT_MIN, WEIGHT_MAX),
    )  # wght
    parser.add_argument(
        "--width",
        default=DEFAULT_WIDTH,
        type=int,
        help="Width axis value ({}-{})".format(WIDTH_MIN, WIDTH_MAX),
    )  # opsz
    parser.add_argument(
        "-v",
        "--version",
        action="version",
        version="vf2s {}".format(SCRIPT_VERSION),
        help="Display application version",
    )
    parser.add_argument("path", help="Variable font path")

    args = parser.parse_args()

    instance_location = {}
    # axis value validity testing and location definitions
    if args.weight is not None:
        if args.weight < WEIGHT_MIN or args.weight > WEIGHT_MAX:
            sys.stderr.write(
                "Weight axis value must be in the range {} - {}{}".format(
                    WEIGHT_MIN, WEIGHT_MAX, os.linesep
                )
            )
            sys.exit(1)
        else:
            instance_location["wght"] = args.weight
    if args.width is not None:
        if args.width < WIDTH_MIN or args.width > WIDTH_MAX:
            sys.stderr.write(
                "Width axis value must be in the range {} - {}{}".format(
                    WIDTH_MIN, WIDTH_MAX, os.linesep
                )
            )
            sys.exit(1)
        else:
            instance_location["wdth"] = args.width

    # variable font path check
    if not os.path.exists(args.path):
        sys.stderr.write(
            "{} does not appear to be a valid path to a variable font{}".format(
                args.path, os.linesep
            )
        )
        sys.exit(1)

    # instantiate the variable font with the requested values
    font = TTFont(args.path)
    instantiateVariableFont(font, instance_location, inplace=True)

    # ---------------------------------------------------------------
    # rewrite name table records with new name values for A/B testing
    # ---------------------------------------------------------------

    namerecord_list = font["name"].names

    # create a name string from the axis location parameters
    axis_param_string = ""
    for axis_value in instance_location:
        axis_param_string += "{}{}".format(axis_value, instance_location[axis_value])

    # map axis name to an abbreviation in font path and name table record string values
    axis_param_string = axis_param_string.replace("wght", "wg")
    axis_param_string = axis_param_string.replace("wdth", "wd")

    # name definitions (NEEDS TO BE MODIFIED TO SUPPORT STYLES OTHER THAN REGULAR)
    nameID1_name = "{} {}".format(FONTNAME, axis_param_string)
    nameID4_name = "{} {} Regular".format(FONTNAME, axis_param_string)
    nameID6_name = "{}-{}-Regular".format(FONTNAME, axis_param_string)
    outfont_name = "{}-{}-Regular.ttf".format(FONTNAME, axis_param_string)
    outfont_path = os.path.join(
        os.path.dirname(os.path.abspath(args.path)), outfont_name
    )

    for record in namerecord_list:
        if record.nameID == 1:
            record.string = nameID1_name
        elif record.nameID == 4:
            record.string = nameID4_name
        elif record.nameID == 6:
            record.string = nameID6_name

    # Set the macOS overlap rendering bit
    # addresses bug in overlap path rendering on macOS web browsers
    font = set_mac_overlap_rendering_bit(font)

    # write the instance font to disk
    try:
        font.save(outfont_path)
        print("[New instance]: {}".format(outfont_path))
    except Exception as e:
        sys.stderr.write(
            "Failed to write font file {} with error: {}{}".format(
                outfont_name, str(e), os.linesep
            )
        )
        sys.exit(1)


if __name__ == "__main__":
    main()
