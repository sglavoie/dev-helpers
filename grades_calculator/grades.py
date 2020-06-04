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
            raise FileNotFoundError("grades.json was not found.")


if __name__ == "__main__":
    GRADES = Grades()
    GRADES.load()
    print(GRADES.grades)
