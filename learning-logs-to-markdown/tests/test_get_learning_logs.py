import sys

sys.path.append(".")

from fixtures.data_original import TEST_DATA
import get_learning_logs as learning


def test_link_is_valid():
    test_cases = [
        {
            "url": "https://docs.google.com/spreadsheets/d/1k9_XOWOoSBWiwiT0ZM4N7wfsl1_wo9Xx8/edit#gid=0",
            "expected": True,
        },
        {
            "url": "https://www.youtube.com/watch?v=Iwf17zsDAnY",
            "expected": True,
        },
        {
            "url": "https://pragprog.com/titles/tpp20/the-pragmatic-programmer-20th-anniversary-edition/",
            "expected": True,
        },
        {
            "url": "https://docs.python.org/3/tutorial/index.html",
            "expected": True,
        },
        {
            "url": "https://.com",
            "expected": False,
        },
        {
            "url": "https://abc.",
            "expected": False,
        },
    ]
    for case in test_cases:
        assert learning.link_is_valid(case["url"]) == case["expected"]


def test_date_is_valid():
    test_cases = [
        {
            "date": "12/3/2021",
            "expected": True,
        },
        {
            "date": "12/31/2021",
            "expected": True,
        },
        {
            "date": "12/32/2021",  # invalid day
            "expected": False,
        },
        {
            "date": "13/30/2021",  # invalid month
            "expected": False,
        },
        {
            "date": "13/30/21",  # invalid year (must be YYYY)
            "expected": False,
        },
        {
            "date": "12.30.2021",  # invalid format (must be mm/dd/yyyy)
            "expected": False,
        },
        {
            "date": "12-30-2021",  # invalid format (must be mm/dd/yyyy)
            "expected": False,
        },
        {
            "date": "string",  # not a date
            "expected": False,
        },
        {
            "date": "",  # an empty value should be considered invalid
            "expected": False,
        },
        {
            "date": None,  # no value should be considered invalid
            "expected": False,
        },
    ]
    for case in test_cases:
        assert learning.date_is_valid(case["date"]) == case["expected"]


def test_row_is_valid():
    for row in TEST_DATA:
        assert learning.row_is_valid(row) == row["Valid"]


def test_data_contains_all_keys_for_all_rows_returns_True_with_all_keys():
    # Just check for the presence of keys (values don't matter here).
    # Keys must match exactly (case-sensitive).
    valid_list = [
        {
            "Date": "",
            "Category": "",
            "Sub-category": "",
            "Title": "",
            "Activity": "",
            "Link": "",
            "Notes": "",
        },
    ]
    assert learning.data_contains_all_keys_for_all_rows(valid_list)


def test_data_contains_all_keys_for_all_rows_returns_False_with_lowercase_key():
    # Test case sensitivity
    invalid_list_with_lowercase_date = [
        {
            "date": "",  # Should be `Date`
            "Category": "",
            "Sub-category": "",
            "Title": "",
            "Activity": "",
            "Link": "",
            "Notes": "",
        },
    ]
    assert (
        learning.data_contains_all_keys_for_all_rows(
            invalid_list_with_lowercase_date
        )
        == False
    )


def test_data_contains_all_keys_for_all_rows_returns_False_on_missing_key():
    # FAIL if a single key is missing
    all_keys = [
        "Date",
        "Category",
        "Sub-category",
        "Title",
        "Activity",
        "Link",
        "Notes",
    ]
    for key in all_keys:
        # Reset `data` before popping each key
        data = {
            "Date": "",
            "Category": "",
            "Sub-category": "",
            "Title": "",
            "Activity": "",
            "Link": "",
            "Notes": "",
        }
        data.pop(key)  # Should now FAIL: all keys must be there
        assert learning.data_contains_all_keys_for_all_rows([data]) == False


def test_data_contains_all_keys_for_all_rows_returns_True_with_extra_key():
    # So as long as all mandatory keys are there, we don't care about
    # extra keys
    data = [
        {
            "Date": "",
            "Category": "",
            "Sub-category": "",
            "Title": "",
            "Activity": "",
            "Link": "",
            "Notes": "",
            "This extra key here": "does not matter",
        }
    ]
    assert learning.data_contains_all_keys_for_all_rows(data)


def test_data_contains_all_keys_for_all_rows_returns_False_with_no_data():
    assert learning.data_contains_all_keys_for_all_rows([]) == False


def test_get_all_years():
    all_years = [2018, 2019, 2020, 2021]
    assert learning.get_all_years(TEST_DATA) == all_years


def test_get_all_months():
    expected_months = [12, 11, 6]
    entries = [e for e in TEST_DATA if "2021" in e["Date"]]
    assert learning.get_all_months(entries) == expected_months


def test_get_string_without_special_characters():
    special_strings = {
        " ### ### #3 ajsdh ## /": "3 ajsdh",
        "*** asdlkj*** ** a": "asdlkj a",
        "asd_asdd___aa____g__": "asd asdd aa g",
        " alkfj\nasd\n\naasd\nf \n": "alkfj asd aasd f",
        "   a\n\n\n ": "a",
        "<h1>some text</h1>": "h1 some text h1",
        "[some  -   text] ": "some - text",
        "{  some text  }": "some text",
        "| ```some text`\\ | ": "some text",
        "- some text": "some text",
    }
    for special_string, expected_string in special_strings.items():
        assert (
            learning.get_string_without_special_characters(special_string)
            == expected_string
        )


def test_get_formatted_title():
    entries = [e for e in TEST_DATA if "LinkMarkdown" in e]
    for entry in entries:
        assert (
            learning.get_formatted_title(entry["Title"], entry["Link"])
            == f'- {entry["LinkMarkdown"]}'
        )


def test_get_formatted_date():
    all_months = {
        1: "January",
        2: "February",
        3: "March",
        4: "April",
        5: "May",
        6: "June",
        7: "July",
        8: "August",
        9: "September",
        10: "October",
        11: "November",
        12: "December",
    }
    for month_num, month_name in all_months.items():
        formatted_month = f"\n\n## {month_name}\n"
        assert learning.get_formatted_date(month_num) == formatted_month


def test_get_formatted_category():
    categories = [
        "Articles",
        "Books",
        "With space",
        "",
    ]
    expected_categories = [
        "- **Articles**",
        "- **Books**",
        "- **With space**",
        "",
    ]
    for category, expected_category in zip(categories, expected_categories):
        assert learning.get_formatted_category(category) == expected_category


def test_get_formatted_sub_category():
    sub_cats = ["Software engineering", "Python", ""]
    expected_sub_cats = [
        "- *Software engineering*",
        "- *Python*",
        "",
    ]
    for sub_cat, expected_sub_cat in zip(sub_cats, expected_sub_cats):
        assert learning.get_formatted_sub_category(sub_cat) == expected_sub_cat


def test_get_formatted_notes():
    notes = ["Note 1", " Note-2  ", ""]
    expected_notes = ["(_Note 1_)", "(_Note-2_)", ""]
    for note, expected_note in zip(notes, expected_notes):
        assert learning.get_formatted_notes(note) == expected_note


def test_get_formatted_activity():
    entries = [
        {"Activity": "", "Notes": "Anything"},
        {"Activity": "Something", "Notes": ""},
        {"Activity": "Some", "Notes": "More things  "},
    ]
    expected_values = ["", "- Something", "- Some (_More things_)"]
    for i, entry in enumerate(entries):
        assert (
            learning.get_formatted_activity(entry["Activity"], entry["Notes"])
            == expected_values[i]
        )


def test_append_activity_appends_activity_when_initially_empty_with_no_notes():
    activity = "Some activity here"
    activities = []

    # added activity without notes
    expected_activities = ["- Some activity here"]
    assert (
        learning.append_activity(activity, "", activities)
        == expected_activities
    )


def test_append_activity_appends_activity_when_initially_empty_with_notes():
    activity = "Some activity here"
    notes = "Some notes here"
    activities = []

    # added activity without notes
    expected_activities = ["- Some activity here (_Some notes here_)"]
    assert (
        learning.append_activity(activity, notes, activities)
        == expected_activities
    )


def test_append_activity_appends_notes_to_activity_without_prior_notes_with_single_activity():
    activity = "Some activity here"
    notes = "Some notes here"
    activities = ["- Some activity here"]  # no notes yet

    # added activity with notes
    expected_activities = ["- Some activity here (_Some notes here_)"]
    assert (
        learning.append_activity(activity, notes, activities)
        == expected_activities
    )


def test_append_activity_appends_notes_to_activity_without_prior_notes_with_multiple_activities():
    activity = "Some activity here"
    notes = "Some notes here"
    activities = [
        "- Some activity here",  # no notes yet
        "- Some other activity",
    ]

    # added activity with notes, removing the old one first
    expected_activities = [
        "- Some other activity",  # Shifted in the existing list
        "- Some activity here (_Some notes here_)",  # Appended with notes
    ]
    assert (
        learning.append_activity(activity, notes, activities)
        == expected_activities
    )


def test_append_activity_does_not_duplicate_activity_with_prior_notes_with_single_activity():
    # Case where same activity and same notes are added again (should not duplicate)
    activity = "Some activity here"
    notes = "Some notes here"
    activities = ["- Some activity here (_Some notes here_)"]
    expected_activities = activities  # same thing
    assert (
        learning.append_activity(activity, notes, activities)
        == expected_activities
    )

    # Case where same activity but without notes is added again
    # (should not modify anything)
    assert (
        learning.append_activity(activity, "", activities)
        == expected_activities
    )

    activity = "Some activity here"
    activities = ["- Some activity here"]  # already formatted, no notes
    expected_activities = ["- Some activity here"]  # no change expected
    assert (
        learning.append_activity(activity, "", activities)
        == expected_activities
    )


def test_append_activity_does_not_duplicate_activity_with_prior_notes_with_multiple_activities():
    # Case where same activity and same notes are added again (should not duplicate)
    activity = "Some activity here"
    notes = "Some notes here"
    activities = [
        "- Some other activity",
        "- Some activity here (_Some notes here_)",
    ]
    expected_activities = activities  # same thing
    assert (
        learning.append_activity(activity, notes, activities)
        == expected_activities
    )

    # Case where same activity but without notes is added again
    # (should not modify anything)
    assert (
        learning.append_activity(activity, "", activities)
        == expected_activities
    )

    activity = "Some activity here"
    activities = [
        "- Some other activity (_With notes_)",
        "- Some activity here (_Some notes here_)",
    ]  # already formatted, no notes
    expected_activities = activities  # no change expected
    assert (
        learning.append_activity(activity, "", activities)
        == expected_activities
    )


def test_append_notes_does_not_duplicate_notes_when_there_is_no_activity():
    notes = "Some notes"
    notes_list = ["Some notes"]
    expected_notes_list = notes_list
    assert learning.append_notes(notes, notes_list) == expected_notes_list

    # Search with multiple items
    notes_list = ["Some other notes", "Some notes"]
    expected_notes_list = notes_list
    assert learning.append_notes(notes, notes_list) == expected_notes_list


def test_append_notes_adds_notes_when_there_is_no_activity_and_notes_werent_there_already():
    notes = "Some notes 2"
    notes_list = ["Some notes"]
    expected_notes_list = ["Some notes", "Some notes 2"]
    assert learning.append_notes(notes, notes_list) == expected_notes_list

    # Search with multiple items
    notes_list = ["Some other notes", "Some notes"]
    expected_notes_list = ["Some other notes", "Some notes", "Some notes 2"]
    assert learning.append_notes(notes, notes_list) == expected_notes_list
