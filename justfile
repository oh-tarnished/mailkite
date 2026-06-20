default:
    @just --list

# run prek hooks (--all-files to force proto lint/generate when no .proto files staged)
prek *args:
    # Auto-fix trailing whitespace and EOF before running hooks
    python3 tools/protobuf/fix-precommit.py .
    prek run --all-files {{ args }}

# Generate protobuf docs (per-module + root README.md) via protodoc.
docs:
    go run ./tools/protobuf/docs protobuf

# Generate the ORM schemas (prisma + gorm). Requires protoc-gen-protorm on PATH (brew/go install).
orm:
    buf generate --template tools/protobuf/buf/orm.buf.gen.yaml

# Merge the per-service generated OpenAPI specs into a single openapiv3-spec.yaml. Requires PyYAML.
openapi-merge:
    python3 tools/protobuf/openapi/main.py

# generated protos for languages. if language is not specified, generates for all supported languages.
generate language="all" descriptors="true":
    #!/usr/bin/env sh
    set -e
    echo "==> Updating buf deps..."
    buf dep update
    echo "==> Language Specific Buf generate..."
    if [ "{{language}}" = "all" ]; then
        buf generate --template tools/protobuf/buf/go.buf.gen.yaml
        buf generate --template tools/protobuf/buf/openapiv3.buf.gen.yaml
        echo "==> OpenAPI merge..."
        python3 tools/protobuf/openapi/main.py
        echo "==> ORM generate (prisma + gorm)..."
        buf generate --template tools/protobuf/buf/orm.buf.gen.yaml
        echo "==> Docs generate..."
        go run ./tools/protobuf/docs protobuf
    elif [ "{{language}}" = "orm" ]; then
        buf generate --template tools/protobuf/buf/orm.buf.gen.yaml
    elif [ "{{language}}" = "docs" ]; then
        go run ./tools/protobuf/docs protobuf
    elif [ "{{language}}" = "openapiv3" ]; then
        buf generate --template tools/protobuf/buf/openapiv3.buf.gen.yaml
        echo "==> OpenAPI merge..."
        python3 tools/protobuf/openapi/main.py
    elif [ -f "tools/protobuf/buf/{{language}}.buf.gen.yaml" ]; then
        buf generate --template tools/protobuf/buf/{{language}}.buf.gen.yaml
    else
        echo "Error: Template for {{language}} not found!"
        echo "Available languages: go, openapiv3, orm, docs"
        exit 1
    fi
