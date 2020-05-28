"""Simple script to get information about progress made in a
BSc Computer Science at the University of London
(calculations are specific to this particular degree)."""

import json
import sys


class Grades:
    """Read data from JSON file to perform different actions on grades,
    such as calculating the GPA."""

    def __init__(self, grades_file="grades.json"):
        """Set data attributes and perform necessary calculations to
        retrieve information."""
        self.total_credits = 0
        # self.calculate_weighted_average()

    def load_grades(self, grades_file="grades.json"):
        try:
            with open(grades_file, "r") as read_file:
                self.grades = json.load(read_file)
        except FileNotFoundError:
            try:
                with open("grades_example.json", "r") as read_file:
                    print("You may want to rename `grades_example.json` to `grades.json`")
                    self.grades = json.load(read_file)
            except FileNotFoundError as e:
                raise(e)

    @staticmethod
    def get_weight_of(level):
        """Return the weight of a given `level`. The ratio is 1:3:5 for
        modules of L4:L5:L6 respectively."""
        if level == 4:
            return 1
        if level == 5:
            return 3
        return 6

    def calculate_weighted_average(self):
        """Calculate the weighted average for all the modules taken so far."""
        scores = []
        weights = []
        for subject_name, subject_details in self.grades.items():
            if subject_name == "Final Project":
                weight = self.get_weight_of(subject_details["level"]) * 2
            else:
                weight = self.get_weight_of(subject_details["level"])
            weights.append(weight)
            scores.append(subject_details["score"] * weight)
        self.scores_average = sum(scores) / sum(weights)

    def get_classification(self):
        """Return a string containing the classification of the student
        according to the Programme Specification."""
        # Per the Programme Specification:
        # https://london.ac.uk/sites/default/files/programme-specifications/progspec-computer-science-2019-2020.PDF
        if self.scores_average >= 70:
            return "First Class Honours"
        if self.scores_average >= 60:
            return "Second Class Honours [Upper Division]"
        if self.scores_average >= 50:
            return "Second Class Honours [Lower Division]"
        if self.scores_average >= 40:
            return "Third Class Honours"
        return "Fail"

    def print_average_grade_percentage(self):
        """Return a string to indicate the final average grade expressed in percentage."""
        print(f"Final average grade: {round(self.scores_average, 2)}%")

    def print_gpa(self, location):
        """Print the GPA depending on the `location`.
        Current values supported for `location`: 'us' and 'uk'."""
        if location == "us":
            result = self.scores_average / 20 - 1
            print(f"GPA as calculated in the US: {round(result, 2)}")
        elif location == "uk":
            # According to Fulbright Commission (NOT an official scale):
            # http://www.fulbright.org.uk/going-to-the-usa/pre-departure/academics
            if self.scores_average < 35:
                result = 0
            if self.scores_average >= 35:
                result = 1.0
            if self.scores_average >= 40:
                result = 2.0
            if self.scores_average >= 45:
                result = 2.3
            if self.scores_average >= 50:
                result = 2.7
            if self.scores_average >= 55:
                result = 3
            if self.scores_average >= 60:
                result = 3.3
            if self.scores_average >= 65:
                result = 3.7
            if self.scores_average >= 70:
                result = 4
            print(
                f"GPA as calculated in the UK: {round(result, 2)} ({self.get_classification()})"
            )

    def print_summary(self):
        """Print a summary to the terminal."""
        self.print_average_grade_percentage()
        self.print_gpa("us")
        self.print_gpa("uk")
        self.print_total_credits()

    def print_total_credits(self):
        """Print a string describing the number of credits done with the total
        number of credits to do and the percentage of credits done so far."""
        self.total_credits = 0
        for subject_name in self.grades.keys():
            if subject_name == "Final Project":
                self.total_credits += 30
            else:
                self.total_credits += 15
        print(
            f"Total credits done: {self.total_credits} out of 360 "
            f"({round(self.total_credits / 360 * 100, 2)}%)"
        )


if __name__ == "__main__":
    GRADES = Grades()
    GRADES.load_grades()
    # GRADES.print_summary()
