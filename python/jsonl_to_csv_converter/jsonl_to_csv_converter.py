import argparse
import csv
import json
from pathlib import Path


def main():
    args = get_parsed_args()
    col_names = get_unique_column_names(args.input_path)
    write_jsonl_to_csv(args.input_path, args.output_path, col_names)


def get_unique_column_names(jsonl_file: Path) -> set[str]:
    col_names = set()
    with open(jsonl_file, "r") as f:
        for line in f:
            json_obj = json.loads(line)
            col_names.update(json_obj.keys())
    return col_names


def get_parsed_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "-i",
        "--input-path",
        type=str,
        required=True,
        help="Path to the JSON Lines file",
    )

    parser.add_argument(
        "-o",
        "--output-path",
        type=str,
        required=True,
        help="Path to the CSV file",
    )
    parsed_args = parser.parse_args()
    parsed_args.input_path = Path(parsed_args.input_path)
    parsed_args.output_path = Path(parsed_args.output_path)

    return parsed_args


def write_jsonl_to_csv(jsonl_file: Path, csv_file: Path, col_names: set[str]) -> None:
    with open(jsonl_file, "r") as jsonl_f, open(csv_file, "w") as csv_f:
        writer = csv.DictWriter(csv_f, fieldnames=sorted(col_names))
        writer.writeheader()
        for line in jsonl_f:
            json_obj = json.loads(line)
            writer.writerow(json_obj)


if __name__ == "__main__":
    main()
