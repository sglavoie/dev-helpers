# Standard library imports
import json
import uuid
from unittest.mock import patch

# Third-party library imports
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
    def test_garbage_json_file_raises_error(grades):
        with pytest.raises(json.decoder.JSONDecodeError):
            grades.load(grades_file="tests/fixtures/json/bad_format.json")

    @staticmethod
    @pytest.mark.parametrize(
        "score,expected_bool",
        [
            (61, True),
            (100, True),
            (0, True),
            (-1, True),
            (101, False),
            ("string", False),
            ({1, 2, 3}, False),
            ({"a": 1}, False),
            ([1, 2, 3], False),
            (-2, False),
            ({}, False),
            ("", False),
            (set(), False),
            (None, False),
        ],
    )
    def test_assert_grades_scores_are_valid(grades, score, expected_bool):
        assert grades.score_is_valid(score) == expected_bool


class TestDataIsRetrievedCorrectly:
    @staticmethod
    @pytest.mark.parametrize(
        "level,expected_weight",
        [
            (4, 1),
            (5, 3),
            (6, 5),
            (2, 0),
            (7, 0),
            (-1, 0),
            (0, 0),
            ("string", 0),
            ("", 0),
            ("1", 0),
            (2.2, 0),
            ([], 0),
            ({}, 0),
            (set(), 0),
        ],
    )
    def test_weight_level_of_module(grades, level, expected_weight):
        """Check that the correct weight is given to each level.
        Level 4 should return 1.
        Level 5 should return 3.
        Level 6 should return 5.
        Anything else should return 0.
        """
        assert grades.get_weight_of(level) == expected_weight

    @staticmethod
    def test_count_finished_modules(grades):
        with patch.dict(
            grades.grades,
            {
                "Module 1": {"score": 100},
                "Module 2": {"score": 0},
                "Module 3": {"score": 80},
                "Module 4": {"score": 75},
                "Module 6": {"score": -1},
                "Module 5": {"level": 4},
                "Module 7": {},
            },
            clear=True,
        ):
            # All are valid except `Module 5` which doesn't have a score
            # `0` means FAILED, `-1` means we got recognition of prior learning
            assert grades.get_num_of_finished_modules() == 5

    @staticmethod
    def test_get_list_of_finished_modules(grades):
        expected_list = [
            {"Module 1": {"score": 100}},
            {"Module 2": {"score": -1}},
            {"Module 3": {"score": 80}},
            {"Module 4": {"score": 75}},
            {"Module 5": {"score": 0}},
        ]
        with patch.dict(
            grades.grades,
            {
                "Module 1": {"score": 100},
                "Module 2": {"score": -1},
                "Module 3": {"score": 80},
                "Module 4": {"score": 75},
                "Module 5": {"score": 0},
                "Module 6": {"level": 4},
                "Module 7": {},
            },
            clear=True,
        ):
            # `0` means the module was FAILED. `-1` means the module was
            # not taken but has been recognized through prior learning, so
            # it is also considered done.
            assert grades.get_list_of_finished_modules() == expected_list

    @staticmethod
    def test_get_scores_of_finished_modules(grades):
        expected_list = [100, 80, 75, 0]
        with patch.dict(
            grades.grades,
            {
                "Module 1": {"score": 100},
                "Module 3": {"score": 80},
                "Module 4": {"score": 75},
                "Module 6": {"score": 0},
                "Module 2": {"score": -1},
                "Module 5": {"level": 4},
                "Module 7": {},
            },
            clear=True,
        ):
            assert grades.get_scores_of_finished_modules() == expected_list


class TestDataIsCalculatedWell:
    @staticmethod
    def test_calculate_average_of_finished_modules_rounds_half_up(grades):
        with patch.dict(
            grades.grades,
            {
                "Module 1": {"score": 100},
                "Module 3": {"score": 80},
                "Module 4": {"score": 79.7},
                "Module 6": {"score": 0},
                "Module 2": {"score": -1},
                "Module 5": {"level": 4},
                "Module 5": {},
            },
            clear=True,
        ):
            assert grades.calculate_average_of_finished_modules() == 64.93
        with patch.dict(
            grades.grades,
            {
                "Module 1": {"score": 97.23},
                "Module 2": {"score": 93.58},
                "Module 3": {"score": 91.11},
                "Module 4": {},
                "Module 5": {"level": 4},
            },
            clear=True,
        ):
            assert grades.calculate_average_of_finished_modules() == 93.97
        with patch.dict(
            grades.grades, {}, clear=True,
        ):
            assert grades.calculate_average_of_finished_modules() == 0
