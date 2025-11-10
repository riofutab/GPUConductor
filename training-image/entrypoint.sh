#!/usr/bin/env bash
set -euo pipefail

DATASET_PATH=${DATASET_PATH:-/workspace/dataset}
MODEL_OUTPUT_PATH=${MODEL_OUTPUT_PATH:-/workspace/output}
TRAINING_SCRIPT=${TRAINING_SCRIPT:-}
ITERATIONS=${TRAINING_ITERATIONS:-1}

mkdir -p "$MODEL_OUTPUT_PATH"

if [[ -f requirements.txt ]]; then
  if [[ ! -d .venv ]]; then
    uv venv .venv
  fi
  source .venv/bin/activate
  uv pip install -r requirements.txt
fi

if [[ -n "$TRAINING_SCRIPT" ]]; then
  echo "Running training script: $TRAINING_SCRIPT"
  bash -c "$TRAINING_SCRIPT"
elif [[ -f train.py ]]; then
  echo "Running train.py with iterations=$ITERATIONS"
  python train.py --iterations "$ITERATIONS" --dataset "$DATASET_PATH" --output "$MODEL_OUTPUT_PATH"
else
  echo "No training script provided" >&2
  exit 1
fi
