
import json
import sys
import unittest

from tracer import (
    trace_execution,
    serialize_value,
    MAX_TRACE_STEPS,
    MAX_CALL_DEPTH,
    MAX_TRACE_BYTES,
    MAX_VALUE_BYTES,
    MAX_COLLECTION_ITEMS
)


class TestTracer(unittest.TestCase):

    # --------------------------------------------------
    # BASIC EXECUTION
    # --------------------------------------------------

    def test_normal_execution(self):

        result = trace_execution("""
x = 10
y = 20
result = x + y
""")

        self.assertEqual(result["status"], "completed")
        self.assertIsNone(result["error"])
        self.assertIsNone(result["limit"])

        final_event = result["events"][-1]

        self.assertEqual(
            final_event["variables"]["result"], 30
        )

    def test_sequential_step_numbers(self):

        result = trace_execution("""
x = 10
x += 5
x *= 2
""")

        steps = [
            event["step"]
            for event in result["events"]
        ]

        self.assertEqual(
            steps,
            list(range(1, len(steps) + 1))
        )

        self.assertEqual(
            result["total_steps"],
            len(result["events"])
        )

    def test_source_line_numbers(self):

        result = trace_execution(
            "x = 10\ny = 20\nresult = x + y"
        )

        lines = [
            event["line"]
            for event in result["events"]
            if event["event"] == "line"
        ]

        self.assertEqual(lines, [1, 2, 3])

    # --------------------------------------------------
    # VARIABLE TRACKING
    # --------------------------------------------------

    def test_variable_creation(self):

        result = trace_execution("x = 10")

        changes = [
            change
            for event in result["events"]
            for change in event.get("changes", [])
        ]

        self.assertIn({
            "type": "variable_created",
            "name": "x",
            "value": 10
        }, changes)

    def test_variable_update(self):

        result = trace_execution("""
x = 10
x = 20
""")

        changes = [
            change
            for event in result["events"]
            for change in event.get("changes", [])
        ]

        self.assertIn({
            "type": "variable_updated",
            "name": "x",
            "old_value": 10,
            "new_value": 20
        }, changes)

    def test_variable_deletion(self):

        result = trace_execution("""
x = 10
del x
""")

        changes = [
            change
            for event in result["events"]
            for change in event.get("changes", [])
        ]

        self.assertIn({
            "type": "variable_deleted",
            "name": "x",
            "old_value": 10
        }, changes)

    # --------------------------------------------------
    # ARRAY TRACKING
    # --------------------------------------------------

    def test_array_append(self):

        result = trace_execution("""
arr = [10, 20, 30]
arr.append(40)
""")

        changes = [
            change
            for event in result["events"]
            for change in event.get("changes", [])
        ]

        self.assertIn({
            "type": "array_updated",
            "name": "arr",
            "old_value": [10, 20, 30],
            "new_value": [10, 20, 30, 40]
        }, changes)

    def test_array_index_update(self):

        result = trace_execution("""
arr = [10, 20, 30]
arr[1] = 99
""")

        final_event = result["events"][-1]

        self.assertEqual(
            final_event["variables"]["arr"],
            [10, 99, 30]
        )

    def test_array_snapshots_are_independent(self):

        result = trace_execution("""
arr = [1, 2]
arr.append(3)
arr.append(4)
""")

        snapshots = [
            event["variables"]["arr"]
            for event in result["events"]
            if "arr" in event.get("variables", {})
        ]

        self.assertIn([1, 2], snapshots)
        self.assertIn([1, 2, 3], snapshots)
        self.assertIn([1, 2, 3, 4], snapshots)

    # --------------------------------------------------
    # FUNCTION TRACKING
    # --------------------------------------------------

    def test_function_call(self):

        result = trace_execution("""
def add(a, b):
    return a + b

result = add(10, 20)
""")

        calls = [
            event
            for event in result["events"]
            if event["event"] == "function_call"
            and event["function"] == "add"
        ]

        self.assertEqual(len(calls), 1)

        self.assertEqual(
            calls[0]["arguments"],
            {"a": 10, "b": 20}
        )

    def test_function_return(self):

        result = trace_execution("""
def add(a, b):
    return a + b

result = add(10, 20)
""")

        returns = [
            event
            for event in result["events"]
            if event["event"] == "function_return"
            and event["function"] == "add"
        ]

        self.assertEqual(len(returns), 1)
        self.assertEqual(returns[0]["return_value"], 30)

    def test_independent_function_frames(self):

        result = trace_execution("""
def first():
    x = 10
    return x

def second():
    x = 20
    return x

a = first()
b = second()
""")

        returns = {
            event["function"]: event
            for event in result["events"]
            if event["event"] == "function_return"
        }

        self.assertEqual(
            returns["first"]["variables"]["x"], 10
        )

        self.assertEqual(
            returns["second"]["variables"]["x"], 20
        )

    # --------------------------------------------------
    # RECURSION AND CALL STACK
    # --------------------------------------------------

    def test_recursive_execution(self):

        result = trace_execution("""
def factorial(n):
    if n <= 1:
        return 1
    return n * factorial(n - 1)

result = factorial(4)
""")

        self.assertEqual(result["status"], "completed")

        self.assertEqual(
            result["events"][-1]["variables"]["result"],
            24
        )

        calls = [
            event
            for event in result["events"]
            if event["event"] == "function_call"
            and event["function"] == "factorial"
        ]

        self.assertEqual(
            [event["depth"] for event in calls],
            [1, 2, 3, 4]
        )

        returns = [
            event["return_value"]
            for event in result["events"]
            if event["event"] == "function_return"
            and event["function"] == "factorial"
        ]

        self.assertEqual(returns, [1, 2, 6, 24])

    def test_module_call_stack(self):

        result = trace_execution("x = 10")

        first_event = result["events"][0]

        self.assertEqual(first_event["depth"], 0)

        self.assertEqual(
            first_event["call_stack"],
            ["<module>"]
        )

    # --------------------------------------------------
    # EXCEPTION HANDLING
    # --------------------------------------------------

    def test_zero_division_error(self):

        result = trace_execution(
            "x = 10\ny = 0\nresult = x / y"
        )

        self.assertEqual(result["status"], "failed")

        self.assertEqual(
            result["error"]["type"],
            "ZeroDivisionError"
        )

        self.assertEqual(result["error"]["line"], 3)

    def test_index_error(self):

        result = trace_execution("""
arr = [1, 2, 3]
result = arr[100]
""")

        self.assertEqual(result["status"], "failed")

        self.assertEqual(
            result["error"]["type"],
            "IndexError"
        )

    def test_name_error(self):

        result = trace_execution(
            "x = undefined_variable"
        )

        self.assertEqual(result["status"], "failed")

        self.assertEqual(
            result["error"]["type"],
            "NameError"
        )

    def test_syntax_error(self):

        result = trace_execution(
            "if x == 5\n    print(x)"
        )

        self.assertEqual(result["status"], "failed")

        self.assertEqual(
            result["error"]["type"],
            "SyntaxError"
        )

        self.assertEqual(result["error"]["line"], 1)
        self.assertEqual(result["total_steps"], 0)

    def test_handled_exception(self):

        result = trace_execution("""
try:
    result = 10 / 0
except ZeroDivisionError:
    result = 0
""")

        self.assertEqual(result["status"], "completed")
        self.assertIsNone(result["error"])

        exception_events = [
            event
            for event in result["events"]
            if event["event"] == "exception"
        ]

        self.assertGreater(len(exception_events), 0)

        self.assertEqual(
            result["events"][-1]["variables"]["result"],
            0
        )

    def test_nested_function_exception(self):

        result = trace_execution("""
def divide(a, b):
    return a / b

def calculate():
    return divide(10, 0)

result = calculate()
""")

        self.assertEqual(result["status"], "failed")

        self.assertEqual(
            result["error"]["function"],
            "divide"
        )

        self.assertEqual(
            result["error"]["type"],
            "ZeroDivisionError"
        )

    def test_preserve_events_after_error(self):

        result = trace_execution("""
x = 10
y = 20
z = x + y
result = z / 0
""")

        self.assertEqual(result["status"], "failed")
        self.assertGreater(result["total_steps"], 0)

        self.assertTrue(
            any(
                event.get("variables", {}).get("z") == 30
                for event in result["events"]
            )
        )

    # --------------------------------------------------
    # SAFETY LIMITS
    # --------------------------------------------------

    def test_maximum_trace_steps(self):

        result = trace_execution("""
x = 0
while True:
    x += 1
""")

        self.assertEqual(
            result["status"],
            "limit_reached"
        )

        self.assertEqual(
            result["limit"],
            "maximum_trace_steps_exceeded"
        )

        self.assertEqual(
            result["total_steps"],
            MAX_TRACE_STEPS
        )

    def test_maximum_call_depth(self):

        result = trace_execution("""
def recurse(n):
    return recurse(n + 1)

recurse(0)
""")

        self.assertEqual(
            result["status"],
            "limit_reached"
        )

        self.assertEqual(
            result["limit"],
            "maximum_call_depth_exceeded"
        )

        maximum_depth = max(
            event["depth"]
            for event in result["events"]
        )

        self.assertEqual(
            maximum_depth,
            MAX_CALL_DEPTH
        )

    def test_maximum_trace_size(self):

        result = trace_execution("""
message = "A" * 60000

for i in range(1000):
    x = i
""")

        self.assertEqual(
            result["status"],
            "limit_reached"
        )

        self.assertEqual(
            result["limit"],
            "maximum_trace_size_exceeded"
        )

        self.assertLessEqual(
            result["trace_size_bytes"],
            MAX_TRACE_BYTES
        )

    # --------------------------------------------------
    # SERIALIZATION
    # --------------------------------------------------

    def test_primitive_serialization(self):

        self.assertEqual(serialize_value(10), 10)
        self.assertEqual(serialize_value(1.5), 1.5)
        self.assertEqual(serialize_value("hello"), "hello")
        self.assertEqual(serialize_value(True), True)
        self.assertIsNone(serialize_value(None))

    def test_large_string_serialization(self):

        value = serialize_value("A" * 200000)

        encoded = json.dumps(
            value,
            ensure_ascii=True,
            separators=(",", ":")
        ).encode("utf-8")

        self.assertLessEqual(
            len(encoded),
            MAX_VALUE_BYTES
        )

        self.assertNotEqual(value, "A" * 200000)

    def test_large_array_serialization(self):

        value = serialize_value(
            list(range(100000))
        )

        self.assertLessEqual(
            len(value),
            MAX_COLLECTION_ITEMS + 1
        )

        self.assertTrue(
            value[-1]["__truncated__"]
        )

    def test_circular_reference(self):

        value = []
        value.append(value)

        serialized = serialize_value(value)

        # Serialization must complete without recursion failure.
        json.dumps(serialized)

        self.assertIsNotNone(serialized)

    def test_non_finite_floats(self):

        self.assertEqual(
            serialize_value(float("inf")),
            "inf"
        )

        self.assertEqual(
            serialize_value(float("nan")),
            "nan"
        )

    # --------------------------------------------------
    # CLEANUP AND JSON COMPATIBILITY
    # --------------------------------------------------

    def test_trace_cleanup(self):

        previous_trace = sys.gettrace()

        trace_execution("x = 10")

        self.assertIs(
            sys.gettrace(),
            previous_trace
        )

    def test_trace_cleanup_after_error(self):

        previous_trace = sys.gettrace()

        trace_execution("x = 10 / 0")

        self.assertIs(
            sys.gettrace(),
            previous_trace
        )

    def test_multiple_independent_executions(self):

        first = trace_execution("x = 10")
        second = trace_execution("y = 20")

        self.assertEqual(first["events"][0]["step"], 1)
        self.assertEqual(second["events"][0]["step"], 1)

        self.assertNotIn(
            "x",
            second["events"][-1]["variables"]
        )

    def test_result_is_json_serializable(self):

        result = trace_execution("""
arr = [1, 2, 3]
data = {"name": "AlgoTrace"}
number = 10
""")

        encoded = json.dumps(
            result,
            allow_nan=False
        )

        decoded = json.loads(encoded)

        self.assertEqual(decoded, result)


if __name__ == "__main__":
    unittest.main(verbosity=2)
