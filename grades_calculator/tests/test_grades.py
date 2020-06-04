# Standard library imports
import uuid

# Third-party library imports
import pytest

# Local imports
from ..grades import Grades

# (level, weight)
LEVELS = [
    (4, 1),
    (5, 3),
    (6, 5),
    ("string", 0),
    ("", 0),
    ("1", 0),
    (2.2, 0),
    ([], 0),
    ({}, 0),
    (set(), 0),
]


@pytest.fixture(scope="module")
def grades():
    return Grades()


class TestGradesAreLoadedProperly:
    @staticmethod
    def test_grades_json_is_loaded_as_dict(grades):
        grades.load()
        assert isinstance(grades.grades, dict)

    @staticmethod
    def test_no_grades_json_raises_file_not_found(grades):
        with pytest.raises(FileNotFoundError):
            non_existent_file = str(uuid.uuid4()) + ".json"
            grades.load(grades_file=non_existent_file)


class TestDataIsRetrievedCorrectly:
    @staticmethod
    @pytest.mark.parametrize("level,expected", LEVELS)
    def test_weight_level_ok(grades, level, expected):
        """Check that the correct weight is given to each level.
        Level 4 should return 1.
        Level 5 should return 3.
        Level 6 should return 5.
        Anything else should return 0.
        """
        assert grades.get_weight_of(level) == expected
