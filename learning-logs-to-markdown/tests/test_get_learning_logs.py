import sys

sys.path.append(".")

from fixtures.data import TEST_DATA
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
