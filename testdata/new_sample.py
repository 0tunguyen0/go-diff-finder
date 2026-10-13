"""New version of the sample module with refactors and fixes."""
import os
import sys
import json


def greet(name, greeting="Hello"):
    message = f"{greeting}, {name}!"
    print(message)
    return message


def compute_total(items, start=0):
    total = start
    for item in items:
        total += item
    return total


def compute_average(items):
    if not items:
        return 0.0
    return compute_total(items) / len(items)


class DataProcessor:
    def __init__(self, source_path, encoding="utf-8"):
        self.source_path = source_path
        self.encoding = encoding
        self.records = []

    def load(self):
        with open(self.source_path, "r", encoding=self.encoding) as handle:
            for line in handle:
                stripped = line.strip()
                if stripped:
                    self.records.append(stripped)

    def summarize(self):
        very_long_summary_line = "processed %d records from %s with status ok, no errors detected anywhere at all" % (
            len(self.records), self.source_path)
        return very_long_summary_line
