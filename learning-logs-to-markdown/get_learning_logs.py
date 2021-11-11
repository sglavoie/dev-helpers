"""
# Data structure:
data = {
    "Month": {  # integer
        "Category": {  # string
            "Sub-category": {  # string
                "Title": {  # string
                    "link": "some_url",  # string if valid
                    "notes": []
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
    "years": [2018, 2019, 2020, 2021],
    "months": [1, 2, 3],
    "categories": ["sorted list of strings"],
    "sub-categories": ["sorted list of strings"],
    "titles": ["sorted list of strings"],
}
months = {1: "January"}  # ...
"""
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
import calendar
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
REGEX_URL = re.compile(
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


def row_is_valid(row: dict) -> bool:
    return date_is_valid(row["Date"]) and row["Title"] != ""


def get_all_years(data: list) -> list:
    years_set = set()
    for row in data:
        if date_is_valid(row["Date"]):
            years_set.add(int(row["Date"][-4:]))
    sorted_years = sorted(list(years_set))
    return sorted_years


def get_all_months(data: list) -> list:
    months_set = set()
    for row in data:
        if date_is_valid(row["Date"]):
            months_set.add(int(row["Date"].split("/")[0]))
    sorted_months = sorted(list(months_set), reverse=True)
    return sorted_months


def date_is_valid(date: str) -> bool:
    try:
        parsed_date = datetime.strptime(date, "%m/%d/%Y")
    except (TypeError, ValueError):
        return False
    return True


def link_is_valid(link: str, url_regex=REGEX_URL) -> bool:
    return re.match(url_regex, link) is not None


def get_formatted_date(month_num: int, render_starting_newlines=True) -> str:
    months = {k: v for k, v in enumerate(calendar.month_name) if k > 0}
    if render_starting_newlines:
        return f"\n\n## {months[month_num]}\n"
    return f"## {months[month_num]}\n"


def get_formatted_category(category: str) -> str:
    if category == "":
        return category
    return f"- **{get_string_without_special_characters(category)}**"


def get_formatted_sub_category(sub_category: str) -> str:
    if sub_category == "":
        return sub_category
    return f"- *{get_string_without_special_characters(sub_category)}*"


def get_formatted_title(title: str, link: str) -> str:
    clean_title = get_string_without_special_characters(title)
    if link_is_valid(link):
        return f"- [{clean_title}]({link})"
    return f"- {clean_title}"


def get_formatted_notes(note: str) -> str:
    if note == "":
        return note
    return f"(_{get_string_without_special_characters(note)}_)"


def get_formatted_activity(activity: str, notes: str) -> str:
    if activity == "":
        return ""
    if notes != "":
        return f"- {activity} {get_formatted_notes(notes)}"
    return f"- {activity}"


def get_string_without_special_characters(
    string: str,
) -> str:
    special_chars = "#*_></`[]{}\\|"
    for char in special_chars:
        string = string.replace(char, " ")

    # Trim superfluous whitespace
    string = string.replace("\n", " ").strip()

    # Avoid getting a string that would result in a bullet list
    if string.startswith("- "):
        string = string[2:]

    # Trim multiple spaces (so we get max 1 consecutive space)
    string = " ".join(string.split())
    return string


def get_data_tree(data: list, months: list) -> tuple:
    tree = {}
    sorted_tree = {}
    sorted_tree_list = {}
    for month in months:
        tree[month] = {}
        sorted_tree[month] = {
            "categories": set(),
            "sub_categories": set(),
            "titles": set(),
        }

        # For each month, add all the data corresponding to that month
        for row in data:
            if int(row["Date"].split("/")[0]) == month:

                # For each month, get all unique categories, sub-categories
                # and titles so we can output them alphabetically
                sorted_tree[month]["categories"].add(row["Category"])
                sorted_tree[month]["sub_categories"].add(row["Sub-category"])
                sorted_tree[month]["titles"].add(row["Title"])

                # For each row, build the nested structure to have
                # Month > Category > Sub-category > Title >
                # link + (activity > notes + activity_message)
                if not tree[month].get(row["Category"]):
                    tree[month][row["Category"]] = {}
                if not tree[month][row["Category"]].get(row["Sub-category"]):
                    tree[month][row["Category"]][row["Sub-category"]] = {}

                # Set default properties inside the title if not there already
                if not tree[month][row["Category"]][row["Sub-category"]].get(
                    row["Title"]
                ):
                    tree[month][row["Category"]][row["Sub-category"]][
                        row["Title"]
                    ] = {"Link": "", "Activities": [], "Notes": []}

                # Override existing link for that title, if there's any
                # and if it's valid
                if link_is_valid(row["Link"]):
                    tree[month][row["Category"]][row["Sub-category"]][
                        row["Title"]
                    ]["Link"] = row["Link"]

                # Append activities in the order they appear in the data
                # if there's one
                if row["Activity"] != "":
                    tree[month][row["Category"]][row["Sub-category"]][
                        row["Title"]
                    ]["Activities"].append(
                        get_formatted_activity(row["Activity"], row["Notes"])
                    )
                # If there's no activity, there can still be a note,
                # keep them all in a list
                elif row["Notes"] != "":
                    tree[month][row["Category"]][row["Sub-category"]][
                        row["Title"]
                    ]["Notes"].append(row["Notes"])

        sorted_tree_list[month] = {
            "categories": sorted(list(sorted_tree[month]["categories"])),
            "sub_categories": sorted(
                list(sorted_tree[month]["sub_categories"])
            ),
            "titles": sorted(list(sorted_tree[month]["titles"])),
        }
    return tree, sorted_tree_list


if __name__ == "__main__":
    data = get_worksheet_data()
    last_year = get_all_years(data)[-1]

    parsed_data = []
    for row in data:
        if row_is_valid(row) and int(row["Date"][-4:]) == last_year:
            parsed_data.append(row)

    months = get_all_months(parsed_data)

    tree, sorted_tree = get_data_tree(parsed_data, months)

    for i, month in enumerate(tree):
        if not i:
            print(get_formatted_date(month, render_starting_newlines=False))
        else:
            print(get_formatted_date(month))

        for category in sorted_tree[month]["categories"]:
            if category in tree[month]:
                if category != "":
                    print(get_formatted_category(category))
                for sub_cat in sorted_tree[month]["sub_categories"]:
                    spaces = ""
                    if sub_cat in tree[month][category]:
                        if sub_cat != "":
                            if category != "":
                                spaces = 4 * " "
                            print(
                                f"{spaces}{get_formatted_sub_category(sub_cat)}"
                            )
                        for title in sorted_tree[month]["titles"]:
                            spaces = ""
                            if title in tree[month][category][sub_cat]:
                                if category != "":
                                    spaces += 4 * " "
                                if sub_cat != "":
                                    spaces += 4 * " "
                                notes = tree[month][category][sub_cat][title][
                                    "Notes"
                                ]
                                if notes == []:
                                    print(
                                        f"{spaces}{get_formatted_title(title, tree[month][category][sub_cat][title]['Link'])}"
                                    )
                                else:
                                    notes_start = "(_"
                                    notes_end = "_)"
                                    concat_notes = "; ".join(notes)
                                    concat_notes = (
                                        notes_start + concat_notes + notes_end
                                    )
                                    print(
                                        f"{spaces}{get_formatted_title(title, tree[month][category][sub_cat][title]['Link'])} "
                                        + concat_notes
                                    )
                                spaces += 4 * " "
                                for activity in tree[month][category][sub_cat][
                                    title
                                ]["Activities"]:
                                    print(f"{spaces}{activity}")
