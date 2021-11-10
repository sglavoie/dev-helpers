"""
# Data structure:
data = {
    "Month": {  # integer
        "Category": {  # string
            "Sub-category": {  # string
                "Title": {  # string
                    "link": "some_url",  # string if valid
                    "activities": [  # list
                        {
                            "note": "Note message",  # formatted string
                            "value": "Activity message",  # string
                        }
                    ],
                }
            }
        }
    }
}
sorted_columns = {
    "months": [1, 2, 3],
    "categories": ["sorted list of strings"],
    "sub-categories": ["sorted list of strings"],
    "titles": ["sorted list of strings"],
}
months = {1: "January"}  # ...
"""

# generate a string for each activity, then concatenate activities together
# generate a string for each title, then concatenate titles together
# generate a string for each title, then concatenate titles together


### Authenticate
# authenticate with API key from file


### Fetch data
# get all rows as dicts


### Sort data chronologically, from oldest to newest
# if no (valid) date, skip

#### Convert all rows into top level with the name of the month (tool will only support output for one year at a time and all dates must fall within the same year)

#### Validate data (discard invalid rows)
##### If no `Title` -> discard


### Parse rows

# start generating string output for each month starting from Dec
# then append each month to a list
# then pop each month from the list to the output

# iterate over sorted months from Jan to Dec
## iterate over sorted categories
### iterate over sorted sub-categories
#### iterate over sorted titles
##### iterate over activities in order

#### If no category -> "0unknown"
#### If no sub-category -> "0unknown"
#### Convert `Title` into link if URL is valid

from datetime import datetime
import re
import os

from dotenv import load_dotenv
import gspread


LEARNING_LOGS_ENV_PATH = os.path.expanduser("~/.learning-logs")
load_dotenv(dotenv_path=LEARNING_LOGS_ENV_PATH)

SERVICE_ACCOUNT_FILEPATH = os.getenv("SERVICE_ACCOUNT_FILEPATH")
SPREADSHEET_ID = os.getenv("SPREADSHEET_ID")
WORKSHEET_ID = os.getenv("WORKSHEET_ID")

# From https://stackoverflow.com/a/7160778/8787680
URL_REGEX = re.compile(
    r"^(?:http|ftp)s?://"  # http:// or https://
    r"(?:(?:[A-Z0-9](?:[A-Z0-9-]{0,61}[A-Z0-9])?\.)+(?:[A-Z]{2,6}\.?|[A-Z0-9-]{2,}\.?)|"  # domain...
    r"localhost|"  # localhost...
    r"\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})"  # ...or ip
    r"(?::\d+)?"  # optional port
    r"(?:/?|[/?]\S+)$",
    re.IGNORECASE,
)


def get_worksheet_data(
    service_account_filename=SERVICE_ACCOUNT_FILEPATH,
    spreadsheet_id=SPREADSHEET_ID,
    worksheet_id=WORKSHEET_ID,
) -> list:
    google_client = gspread.service_account(filename=service_account_filename)
    spreadsheet = google_client.open_by_key(spreadsheet_id)
    worksheet = spreadsheet.get_worksheet_by_id(int(worksheet_id))
    list_of_dicts = worksheet.get_all_records()
    return list_of_dicts


def link_is_valid(link: str, url_regex=URL_REGEX) -> bool:
    return re.match(url_regex, link) is not None


def row_is_valid(row: dict) -> bool:
    return date_is_valid(row["Date"]) and row["Title"] != ""


def date_is_valid(date: str) -> bool:
    try:
        parsed_date = datetime.strptime(date, "%m/%d/%Y")
    except (TypeError, ValueError):
        return False
    return True


if __name__ == "__main__":
    from tests.fixtures.data import TEST_DATA
    from pprint import pprint

    pprint(TEST_DATA)
