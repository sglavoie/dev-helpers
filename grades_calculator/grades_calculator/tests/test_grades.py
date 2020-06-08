# Standard library imports
import json
import uuid

# Third-party library imports
from mock import patch
import pytest


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


class TestJsonStructureIsFormattedWell:
    @staticmethod
    def test_json_file_being_garbage_raises_error(grades):
        with pytest.raises(json.decoder.JSONDecodeError):
            grades.load(grades_file="tests/fixtures/json/bad_format.json")

    @staticmethod
    def test_assert_modules_have_a_valid_score(grades):
        with patch.dict(
            grades.grades,
            {"Module name 1": {"score": 61}, "Module name 2": {"score": 70}},
            clear=True,
        ):
            assert grades.scores_are_valid()

    @staticmethod
    def test_assert_grades_are_missing_scores(grades):
        with patch.dict(
            grades.grades,
            {"Module name 1": {"score": 61}, "Module name 2": {"level": 4}},
            clear=True,
        ):
            assert not grades.scores_are_valid()

    @staticmethod
    def test_assert_grades_have_invalid_scores(grades):
        with patch.dict(
            grades.grades,
            {
                "Module name 1": {"score": 61},
                "Module name 2": {"score": 101},
                "Module name 3": {"score": -1},
                "Module name 4": {"score": "abc"},
                "Module name 5": {"score": {1, 2, 3}},
                "Module name 6": {"score": {"a": 1}},
                "Module name 7": {"score": [1, 2, 3]},
            },
            clear=True,
        ):
            assert not grades.scores_are_valid()


class TestDataIsRetrievedCorrectly:
    @staticmethod
    @pytest.mark.parametrize(
        "level,expected_weight",
        [
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
        ],
    )
    def test_weight_level_ok(grades, level, expected_weight):
        """Check that the correct weight is given to each level.
        Level 4 should return 1.
        Level 5 should return 3.
        Level 6 should return 5.
        Anything else should return 0.
        """
        assert grades.get_weight_of(level) == expected_weight
