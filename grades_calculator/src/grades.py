"""Simple script to get information about progress made in a
BSc Computer Science at the University of London
(calculations are specific to this particular degree)."""

# Standard library imports
import json

# Local imports
from utils import mathtools


class Grades:
    def __init__(self) -> None:
        self.grades = None

    def load(self, grades_file="grades.json") -> None:
        try:
            with open(grades_file) as grades_json:
                self.grades = json.load(grades_json)
        except FileNotFoundError:
            raise FileNotFoundError(f"\n\n{grades_file} was not found.\n")
        except json.decoder.JSONDecodeError as err:
            print(f"\n\n{grades_file} is not formatted correctly.\n")
            raise err

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

    @staticmethod
    def score_is_valid(score) -> bool:
        try:
            if (
                score is not None
                and isinstance(float(score), float)
                and (0 <= score <= 100 or score == -1)
            ):
                return True
        except (ValueError, TypeError):
            pass
        return False

    def get_num_of_finished_modules(self) -> int:
        total = 0
        for _, values in self.grades.items():
            score = values.get("score")
            if self.score_is_valid(score):
                total += 1
        return total

    def get_list_of_finished_modules(self) -> list:
        modules = []
        for module, values in self.grades.items():
            score = values.get("score")
            if self.score_is_valid(score):
                modules.append({module: values})
        return modules

    def get_scores_of_finished_modules(self) -> list:
        modules = self.get_list_of_finished_modules()
        scores = []
        for module in modules:
            for value in module.values():
                score = value.get("score")
                if "score" in value and score >= 0:
                    scores.append(score)
        return scores

    def calculate_average_of_finished_modules(self) -> float:
        scores = self.get_scores_of_finished_modules()
        if len(scores) == 0:
            return 0
        return mathtools.round_half_up(sum(scores) / len(scores), 2)

    def get_classification(self) -> str:
        """Return a string containing the classification of the student
        according to the Programme Specification."""
        score = self.calculate_average_of_finished_modules()
        if score >= 70:
            return "First Class Honours"
        if score >= 60:
            return "Second Class Honours [Upper Division]"
        if score >= 50:
            return "Second Class Honours [Lower Division]"
        if score >= 40:
            return "Third Class Honours"
        return "Fail"
    
    def get_uk_gpa(self) -> float:
        """Return the GPA as calculated in the UK."""
        score = self.calculate_average_of_finished_modules()
        if score < 35:
            result = 0
        if score >= 35:
            result = 1.0
        if score >= 40:
            result = 2.0
        if score >= 45:
            result = 2.3
        if score >= 50:
            result = 2.7
        if score >= 55:
            result = 3
        if score >= 60:
            result = 3.3
        if score >= 65:
            result = 3.7
        if score >= 70:
            result = 4
        return round(result, 2)
    
    def get_us_gpa(self) -> float:
        """Return the GPA as calculated in the US."""
        score = self.calculate_average_of_finished_modules()
        result = round(score / 20 -1, 2)
        if result >= 0:
            return result
        return 0




if __name__ == "__main__":
    GRADES = Grades()
    GRADES.load()
    print("Modules taken:", GRADES.get_list_of_finished_modules())
    print("Number of modules done:", GRADES.get_num_of_finished_modules())
    print("Scores so far:", GRADES.get_scores_of_finished_modules())
    print("Average so far:", GRADES.calculate_average_of_finished_modules())
    print("Classification:", GRADES.get_classification())
