#!/usr/bin/env python3
"""Normalize a saved PortMaster device_info.json; never infers tested support."""
import argparse
import datetime
import json


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input", help="saved PortMaster device_info.json")
    parser.add_argument("output", help="normalized target matrix")
    parser.add_argument("--date", required=True, help="snapshot retrieval date, YYYY-MM-DD")
    args = parser.parse_args()
    datetime.date.fromisoformat(args.date)
    with open(args.input, encoding="utf-8") as source:
        data = json.load(source)
    rows = []
    for device, firmwares in sorted(data.items()):
        for firmware, info in sorted(firmwares.items()):
            if info.get("ram") != 1024:
                continue
            rows.append({
                "device": device, "firmware": firmware,
                "cpu": info.get("cpu"), "architecture": info.get("primary_arch"),
                "glibc": info.get("glibc"), "resolution": info.get("resolution"),
                "opengl": "opengl" in info.get("capabilities", []),
                "verification": "not tested",
            })
    if not rows:
        parser.error("no devices with ram=1024 in input snapshot")
    with open(args.output, "w", encoding="utf-8") as output:
        json.dump({
            "source": "https://github.com/PortsMaster/PortMaster-Info/blob/main/device_info.json",
            "retrieved": args.date, "devices": rows,
        }, output, ensure_ascii=False, indent=2)
        output.write("\n")


if __name__ == "__main__":
    main()
