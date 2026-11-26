import unittest

from object_tracker import ObjectTracker
from tracer import trace_execution


class Node:

    def __init__(self, value):

        self.value = value
        self.next = None


class ObjectTrackerTests(unittest.TestCase):

    def setUp(self):

        self.tracker = ObjectTracker()

    def test_shared_reference(self):

        a = [1, 2, 3]
        b = a

        result = self.tracker.snapshot({
            "a": a,
            "b": b
        })

        self.assertEqual(
            result["references"]["a"],
            result["references"]["b"]
        )

    def test_distinct_objects(self):

        result = self.tracker.snapshot({
            "a": [1, 2],
            "b": [1, 2]
        })

        self.assertNotEqual(
            result["references"]["a"],
            result["references"]["b"]
        )

    def test_linked_list(self):

        head = Node(10)
        head.next = Node(20)

        result = self.tracker.snapshot({
            "head": head
        })

        head_id = result["references"]["head"]

        next_id = (
            result["objects"][head_id]
            ["attributes"]["next"]["$ref"]
        )

        self.assertEqual(
            result["objects"][next_id]
            ["attributes"]["value"],
            20
        )

    def test_circular_reference(self):

        node = Node(10)
        node.next = node

        result = self.tracker.snapshot({
            "node": node
        })

        node_id = result["references"]["node"]

        self.assertEqual(
            result["objects"][node_id]
            ["attributes"]["next"]["$ref"],
            node_id
        )

    def test_stable_identity(self):

        node = Node(10)

        first = self.tracker.snapshot({
            "node": node
        })

        node.value = 100

        second = self.tracker.snapshot({
            "node": node
        })

        self.assertEqual(
            first["references"]["node"],
            second["references"]["node"]
        )

    def test_reference_reassignment(self):

        first_node = Node(10)
        second_node = Node(20)

        first = self.tracker.snapshot({
            "temp": first_node
        })

        second = self.tracker.snapshot({
            "temp": second_node
        })

        self.assertNotEqual(
            first["references"]["temp"],
            second["references"]["temp"]
        )

    def test_snapshot_immutability(self):

        node = Node(10)

        first = self.tracker.snapshot({
            "node": node
        })

        node.value = 50

        node_id = first["references"]["node"]

        self.assertEqual(
            first["objects"][node_id]
            ["attributes"]["value"],
            10
        )

    def test_property_not_executed(self):

        calls = []

        class Dangerous:

            @property
            def __dict__(self):

                calls.append(True)

                raise AssertionError(
                    "Property must not execute"
                )

        result = self.tracker.snapshot({
            "value": Dangerous()
        })

        self.assertEqual(calls, [])

        self.assertEqual(
            result["references"],
            {}
        )

    def test_reference_deletion(self):

        node = Node(10)

        first = self.tracker.snapshot({
            "a": node,
            "b": node
        })

        second = self.tracker.snapshot({
            "b": node
        })

        self.assertNotIn(
            "a",
            second["references"]
        )

        self.assertEqual(
            first["references"]["b"],
            second["references"]["b"]
        )

    def test_trace_integration(self):

        result = trace_execution("""
class Node:
    def __init__(self, value):
        self.value = value
        self.next = None

head = Node(10)
head.next = Node(20)
temp = head.next
""")

        self.assertEqual(
            result["status"],
            "completed"
        )

        last_event = result["events"][-1]

        head_id = last_event["references"]["head"]
        temp_id = last_event["references"]["temp"]

        self.assertEqual(
            last_event["objects"][head_id]
            ["attributes"]["next"]["$ref"],
            temp_id
        )

    def test_all_event_types(self):

        result = trace_execution("""
def calculate():
    a = [1, 2]

    try:
        x = 1 / 0
    except ZeroDivisionError:
        a.append(3)

    return a

result = calculate()
""")

        self.assertEqual(
            result["status"],
            "completed"
        )

        event_types = set()

        for event in result["events"]:

            event_types.add(event["event"])

            self.assertIn("references", event)
            self.assertIn("objects", event)
            self.assertIn("objects_truncated", event)

        self.assertTrue({
            "function_call",
            "line",
            "exception",
            "function_return"
        }.issubset(event_types))


if __name__ == "__main__":

    unittest.main()