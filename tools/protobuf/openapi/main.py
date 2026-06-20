import argparse
import glob
import logging
import os
import re

import yaml

logger = logging.getLogger("openapi-merge")

# Resolve paths relative to the repo root so the script runs from any cwd.
# tools/protobuf/openapi/main.py -> repo root is three directories up.
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(SCRIPT_DIR)))
DEFAULT_SPEC_DIR = os.path.join(REPO_ROOT, "protobuf", "generated", "openapi")
DEFAULT_OUTPUT = os.path.join(DEFAULT_SPEC_DIR, "openapiv3-spec.yaml")

merged = {
    "openapi": "3.0.0",
    "info": {
        "title": "mailkite API",
        "summary": "Scheduling, availability, and booking platform API.",
        "description": (
            "mailkite is a scheduling platform that exposes resources, schedules, "
            "availability windows, bookings, organisations, and promotional codes "
            "over a RESTful, OpenAPI-described surface. This document is generated "
            "from Protocol Buffer service definitions and merged into a single "
            "specification.\n\n"
            "Authentication is performed via bearer tokens unless otherwise noted. "
            "All timestamps are RFC 3339 and all monetary amounts follow the "
            "associated currency's minor units."
        ),
        "version": "1.0.0",
        "termsOfService": "https://mailkite.oh-tarnished.dev/terms",
        "contact": {
            "name": "mailkite API Support",
            "url": "https://mailkite.oh-tarnished.dev/support",
            "email": "support@oh-tarnished.dev",
        },
        "license": {
            "name": "Apache 2.0",
            "url": "https://www.apache.org/licenses/LICENSE-2.0",
        },
    },
    "externalDocs": {
        "description": "mailkite API documentation",
        "url": "https://mailkite.oh-tarnished.dev/docs",
    },
    "servers": [
        {"url": "https://mailkite.oh-tarnished.dev", "description": "Production"},
        {"url": "https://staging.mailkite.oh-tarnished.dev", "description": "Staging"},
        {"url": "http://localhost:8080", "description": "Local development"},
    ],
    "security": [{"bearerAuth": []}],
    "tags": [],
    "paths": {},
    "components": {
        "securitySchemes": {
            "bearerAuth": {
                "type": "http",
                "scheme": "bearer",
                "bearerFormat": "JWT",
                "description": "Bearer token authentication using a JWT access token.",
            }
        }
    },
}


def split_camel_case(name):
    """Insert spaces at camelCase boundaries, e.g. 'BookingService' -> 'Booking Service'."""
    return re.sub(r'(?<=[a-z0-9])(?=[A-Z])', ' ', name)


def humanize_operation_id(operation_id):
    """Convert 'ServiceName_MethodName' to 'Method Name'."""
    if not operation_id:
        return None
    parts = operation_id.split("_", 1)
    return split_camel_case(parts[-1])


def patch_operations(paths):
    """Add summary and 4xx response to operations that are missing them."""
    http_methods = {"get", "post", "put", "patch", "delete", "head", "options", "trace"}
    for path, path_item in paths.items():
        if not isinstance(path_item, dict):
            continue
        for method in http_methods:
            op = path_item.get(method)
            if not isinstance(op, dict):
                continue
            if isinstance(op.get("tags"), list):
                op["tags"] = [split_camel_case(tag) for tag in op["tags"]]
            if "summary" not in op:
                op["summary"] = humanize_operation_id(op.get("operationId")) or method.upper() + " " + path
            responses = op.get("responses", {})
            has_4xx = any(str(code).startswith("4") for code in responses)
            if not has_4xx:
                responses["400"] = {
                    "description": "Bad Request",
                    "content": {
                        "application/json": {
                            "schema": {"$ref": "#/components/schemas/Status"}
                        }
                    }
                }
                op["responses"] = responses


def merge_component_sections(target, source, prefer_strong=False):
    for section, content in source.items():
        if not isinstance(content, dict):
            continue
        if section not in target:
            target[section] = {}

        for key, value in content.items():
            if key in target[section]:
                if prefer_strong:
                    logger.debug("Overriding component '%s' in '%s' with strong type definition", key, section)
                else:
                    logger.debug("Duplicate component '%s' in '%s'; keeping existing definition", key, section)
                    continue
            target[section][key] = value


def merge_paths_and_weak_components(spec_dir):
    """Load all *.openapi.yaml files (for paths and weak components)."""
    pattern = os.path.join(spec_dir, "**", "*.openapi.yaml")
    files = sorted(glob.glob(pattern, recursive=True))
    if not files:
        logger.warning("No *.openapi.yaml files found under %s", spec_dir)
    for file in files:
        with open(file, "r") as f:
            try:
                doc = yaml.safe_load(f)
            except yaml.YAMLError as e:
                logger.error("Failed to parse %s: %s", file, e)
                continue

            if not isinstance(doc, dict):
                logger.warning("Skipping non-object YAML: %s", file)
                continue

            logger.info("Merging %s", os.path.relpath(file, spec_dir))

            # Merge paths
            if isinstance(doc.get("paths"), dict):
                for path, path_item in doc["paths"].items():
                    if path in merged["paths"]:
                        logger.warning("Duplicate path '%s'; keeping last loaded", path)
                    merged["paths"][path] = path_item

            # Merge weak components
            if isinstance(doc.get("components"), dict):
                merge_component_sections(merged["components"], doc["components"], prefer_strong=False)


def merge_strong_components(spec_dir):
    """Load strong component definitions from types/*.yaml."""
    pattern = os.path.join(spec_dir, "types", "*.yaml")
    for file in sorted(glob.glob(pattern)):
        with open(file, "r") as f:
            try:
                doc = yaml.safe_load(f)
            except yaml.YAMLError as e:
                logger.error("Failed to parse type file %s: %s", file, e)
                continue

            if not isinstance(doc, dict) or "components" not in doc:
                logger.debug("Skipping non-component file: %s", os.path.basename(file))
                continue

            logger.info("Loading strong components from %s", os.path.basename(file))
            merge_component_sections(merged["components"], doc["components"], prefer_strong=True)


def collect_tags(paths):
    """Build the top-level tags list from the tags used across operations."""
    http_methods = {"get", "post", "put", "patch", "delete", "head", "options", "trace"}
    names = set()
    for path_item in paths.values():
        if not isinstance(path_item, dict):
            continue
        for method in http_methods:
            op = path_item.get(method)
            if isinstance(op, dict):
                names.update(op.get("tags", []))
    return [{"name": name} for name in sorted(names)]


def find_refs(obj):
    """Recursively find all $ref targets in the spec."""
    refs = set()
    if isinstance(obj, dict):
        if "$ref" in obj:
            refs.add(obj["$ref"])
        for v in obj.values():
            refs.update(find_refs(v))
    elif isinstance(obj, list):
        for item in obj:
            refs.update(find_refs(item))
    return refs


def prune_unused_components(spec):
    """Remove component schemas not referenced anywhere."""
    schemas = spec.get("components", {}).get("schemas", {})
    if not schemas:
        return

    def referenced_names():
        all_refs = find_refs(spec.get("paths", {}))
        # Also check cross-references within components themselves.
        all_refs.update(find_refs(spec.get("components", {})))
        return {
            ref.split("/")[-1]
            for ref in all_refs
            if ref.startswith("#/components/schemas/")
        }

    # Iteratively prune since removing a schema may make others unused.
    removed_total = 0
    unused = set(schemas.keys()) - referenced_names()
    while unused:
        for name in unused:
            del schemas[name]
        removed_total += len(unused)
        unused = set(schemas.keys()) - referenced_names()
    if removed_total:
        logger.info("Pruned %d unreferenced component schema(s)", removed_total)


def main():
    parser = argparse.ArgumentParser(
        description="Merge generated per-service OpenAPI specs into a single spec."
    )
    parser.add_argument(
        "spec_dir",
        nargs="?",
        default=DEFAULT_SPEC_DIR,
        help=f"Directory containing generated OpenAPI specs (default: {DEFAULT_SPEC_DIR})",
    )
    parser.add_argument(
        "-o", "--output",
        default=DEFAULT_OUTPUT,
        help=f"Path to write the merged spec (default: {DEFAULT_OUTPUT})",
    )
    parser.add_argument(
        "-v", "--verbose",
        action="store_true",
        help="Enable verbose (debug) logging.",
    )
    args = parser.parse_args()

    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s %(levelname)-7s %(message)s",
        datefmt="%H:%M:%S",
    )

    spec_dir = os.path.abspath(args.spec_dir)
    if not os.path.isdir(spec_dir):
        parser.error(f"Spec directory does not exist: {spec_dir}")

    # Step 1: paths + weak components from *.openapi.yaml.
    merge_paths_and_weak_components(spec_dir)
    # Step 2: strong component definitions from types/*.yaml.
    merge_strong_components(spec_dir)
    # Step 3: patch operations with summary and 4xx responses.
    patch_operations(merged["paths"])
    # Step 4: populate the top-level tags list from operation tags.
    merged["tags"] = collect_tags(merged["paths"])
    # Step 5: drop components nothing references.
    prune_unused_components(merged)

    # Step 6: write the merged output.
    output = os.path.abspath(args.output)
    os.makedirs(os.path.dirname(output), exist_ok=True)
    with open(output, "w") as f:
        yaml.dump(merged, f, sort_keys=False)

    logger.info(
        "Merged spec written to %s (%d paths, %d schemas, %d tags)",
        output,
        len(merged["paths"]),
        len(merged.get("components", {}).get("schemas", {})),
        len(merged["tags"]),
    )


if __name__ == "__main__":
    main()
