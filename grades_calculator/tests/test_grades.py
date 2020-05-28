import os

from ..grades import Grades

import pytest

# (level, weight)
LEVELS = [
    (4, 1),
    (5, 3),
    (6, 6),
]


@pytest.fixture(scope="module")
def grades():
    return Grades()


@pytest.fixture()
def ex_grades():
    grades = Grades()
    grades.load_grades(grades_file="grades_example.json")
    return grades


class TestClass:
    def test_init_values(self, grades):
        """Check that initial values are correctly set in the class."""
        assert grades.total_credits == 0

    @pytest.mark.parametrize("level,expected", LEVELS)
    def test_weight_level_ok(self, grades, level, expected):
        """Check that the correct weight is given to each level.
        Level 4 should return 1.
        Level 5 should return 3.
        Level 6 should return 6.
        """
        assert grades.get_weight_of(level) == expected

    def test_load_grades_example(self, grades):
        grades.load_grades(grades_file="grades_example.json")
        assert grades.grades["Module name 1"]["score"] == 61
        assert grades.grades["Module name 1"]["level"] == 4
        assert grades.grades["Final Project"]["score"] == 70.8
        assert grades.grades["Final Project"]["level"] == 6

    # TODO: This is wrong. This conditional statement means
    # that the assert is not always run.
    def test_load_grades_file_not_found(self, grades, ex_grades):
        """Check that if `grades.json` is not there, then
        `grades_example.json` is being loaded instead."""
        grades.load_grades(grades_file="grades.json")
        if not os.path.exists("grades.json"):
            assert grades.grades == ex_grades.grades
