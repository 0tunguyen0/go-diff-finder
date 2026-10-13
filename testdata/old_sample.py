"""Old version of the sample module."""
import os
import sys


def greet(name):
    message = "Hello, " + name + "!"
    print(message)
    return message


def compute_total(items):
    total = 0
    for item in items:
        total += item
    return total


class DataProcessor:
    def __init__(self, source_path):
        self.source_path = source_path
        self.records = []

    def load(self):
        with open(self.source_path, "r", encoding="utf-8") as handle:
            for line in handle:
                self.records.append(line.strip())

    def summarize(self):
        very_long_summary_line = "processed %d records from %s with status ok and no errors detected anywhere" % (
            len(self.records), self.source_path)
        return very_long_summary_line
