"""Simple script to get information about progress made in a
BSc Computer Science at the University of London
(calculations are specific to this particular degree)."""

import json


class Grades:
    def __init__(self):
        self.grades = None

    @staticmethod
    def get_weight_of(level: int) -> int:
        """Return the weight of a given `level`. The ratio is 1:3:5 for
        modules of L4:L5:L6 respectively."""
        if level == 4:
            return 1
        if level == 5:
            return 3
        if level == 6:
            return 5
        return 0

    def load(self, grades_file="grades.json"):
        try:
            with open(grades_file) as grades_json:
                self.grades = json.load(grades_json)
        except FileNotFoundError:
            raise FileNotFoundError(f"\n\n{grades_file} was not found.\n")
        except json.decoder.JSONDecodeError as err:
            print(f"\n\n{grades_file} is not formatted correctly.\n")
            raise err

    def scores_are_valid(self):
        for _, values in self.grades.items():
            try:
                if not isinstance(float(values["score"]), float):
                    return False
            except (KeyError, TypeError, ValueError):
                return False
        return True


if __name__ == "__main__":
    GRADES = Grades()
    GRADES.load()
    print(GRADES.grades)
