import sys
import json
import types
import math
import time
from object_tracker import ObjectTracker

# --------------------------------------------------
# TRACE SAFETY LIMITS
# --------------------------------------------------

MAX_TRACE_STEPS = 10_000
MAX_TRACE_SECONDS = 10.0
MAX_CALL_DEPTH = 100

MAX_VALUE_BYTES = 64 * 1024
MAX_TRACE_BYTES = 5 * 1024 * 1024

MAX_COLLECTION_ITEMS = 500
MAX_SERIALIZATION_DEPTH = 8


class TraceLimitExceeded(BaseException):
    """Raised when a tracing safety limit is exceeded."""
    pass


# --------------------------------------------------
# SAFE VALUE SERIALIZATION
# --------------------------------------------------

def serialize_value(value, depth=0, budget=None):

    if budget is None:
        budget = [MAX_VALUE_BYTES]

    def marker(reason):
        return {
            "__truncated__": True,
            "reason": reason
        }

    if depth > MAX_SERIALIZATION_DEPTH:
        return marker("maximum_serialization_depth_exceeded")

    if budget[0] <= 16:
        return marker("maximum_value_size_exceeded")

    if value is None or type(value) is bool:
        result = value

    elif type(value) is int:
        if value.bit_length() > 4096:
            result = marker("maximum_integer_size_exceeded")
        else:
            result = value

    elif type(value) is float:
        result = value if math.isfinite(value) else str(value)

    elif type(value) is str:

        available = max(0, budget[0] - 32)

        encoded = value[:available].encode(
            "utf-8", errors="replace"
        )

        if len(value) > available or len(encoded) > available:
            result = (
                encoded[:available].decode(
                    "utf-8", errors="ignore"
                )
                + "...[truncated]"
            )
        else:
            result = value

    elif isinstance(value, types.FunctionType):
        result = f"<function {value.__name__}>"

    elif type(value) in (list, tuple, set, frozenset):

        result = []

        for index, item in enumerate(value):

            if index >= MAX_COLLECTION_ITEMS:
                result.append(
                    marker("maximum_collection_items_exceeded")
                )
                break

            if budget[0] <= 32:
                result.append(
                    marker("maximum_value_size_exceeded")
                )
                break

            result.append(
                serialize_value(item, depth + 1, budget)
            )

    elif type(value) is dict:

        result = {}

        for index, (key, val) in enumerate(value.items()):

            if index >= MAX_COLLECTION_ITEMS:
                result["__truncated__"] = (
                    "maximum_collection_items_exceeded"
                )
                break

            if budget[0] <= 32:
                result["__truncated__"] = (
                    "maximum_value_size_exceeded"
                )
                break

            if type(key) is str:
                safe_key = key[:256]
            elif type(key) in (int, float, bool, type(None)):
                safe_key = str(
                    serialize_value(key)
                )[:256]
            else:
                safe_key = f"<key {index}>"

            result[safe_key] = serialize_value(
                val,
                depth + 1,
                budget
            )

    else:
        # Avoid executing arbitrary user-defined __repr__ methods.
        result = f"<object {type(value).__name__}>"

    encoded = json.dumps(
        result,
        ensure_ascii=True,
        separators=(",", ":"),
        allow_nan=False
    ).encode("utf-8")

    if len(encoded) > MAX_VALUE_BYTES:
        return marker("maximum_value_size_exceeded")

    budget[0] -= len(encoded)

    return result


# --------------------------------------------------
# VARIABLE SNAPSHOTS
# --------------------------------------------------

def capture_variables(frame):

    return {
        key: serialize_value(value)
        for key, value in frame.f_locals.items()
        if not key.startswith("__")
    }


# --------------------------------------------------
# VARIABLE CHANGE DETECTION
# --------------------------------------------------

def detect_changes(previous_variables, current_variables):

    changes = []

    # Created and updated variables.
    for name, value in current_variables.items():

        if name not in previous_variables:

            changes.append({
                "type": "variable_created",
                "name": name,
                "value": value
            })

        elif previous_variables[name] != value:

            old_value = previous_variables[name]

            if (
                isinstance(value, list)
                and isinstance(old_value, list)
            ):
                change_type = "array_updated"
            else:
                change_type = "variable_updated"

            changes.append({
                "type": change_type,
                "name": name,
                "old_value": old_value,
                "new_value": value
            })

    # Deleted variables.
    for name, old_value in previous_variables.items():

        if name not in current_variables:

            changes.append({
                "type": "variable_deleted",
                "name": name,
                "old_value": old_value
            })

    return changes


# --------------------------------------------------
# ERROR INFORMATION
# --------------------------------------------------

def exception_message(exception):

    try:
        return str(exception)[:4096]
    except Exception:
        return "Unable to serialize exception message"


def build_error(exception):

    error = {
        "type": type(exception).__name__,
        "message": exception_message(exception),
        "line": None,
        "function": None
    }

    # Syntax errors occur before execution starts.
    if isinstance(exception, SyntaxError):

        error["line"] = exception.lineno
        error["column"] = exception.offset
        error["function"] = "<module>"

        return error

    # Find the deepest user-code frame in the traceback.
    traceback_obj = exception.__traceback__

    while traceback_obj is not None:

        frame = traceback_obj.tb_frame

        if frame.f_code.co_filename == "<user_code>":

            error["line"] = traceback_obj.tb_lineno
            error["function"] = frame.f_code.co_name

        traceback_obj = traceback_obj.tb_next

    return error


# --------------------------------------------------
# EXECUTION TRACER
# --------------------------------------------------

def trace_execution(code):

    deadline = time.monotonic() + MAX_TRACE_SECONDS
    events = []
    frame_states = {}

    call_stack = []
    object_tracker = ObjectTracker()

    limit_reached = None
    trace_bytes = 0
    execution_error = None

    # --------------------------------------------------
    # EVENT RECORDER
    # --------------------------------------------------

        # --------------------------------------------------
    # EVENT RECORDER
    # --------------------------------------------------

    def record_event(event_data, frame):

        nonlocal trace_bytes, limit_reached

        if len(events) >= MAX_TRACE_STEPS:

            limit_reached = "maximum_trace_steps_exceeded"

            raise TraceLimitExceeded()

        # Capture object identities and relationships.
        object_snapshot = object_tracker.snapshot(
            frame.f_locals
        )

        # Add object information to the existing event.
        event_data.update(object_snapshot)

        encoded = json.dumps(
            event_data,
            ensure_ascii=True,
            separators=(",", ":"),
            allow_nan=False
        ).encode("utf-8")

        event_size = len(encoded) + 1

        if trace_bytes + event_size > MAX_TRACE_BYTES:

            limit_reached = "maximum_trace_size_exceeded"

            raise TraceLimitExceeded()

        events.append(event_data)

        trace_bytes += event_size

        

    # --------------------------------------------------
    # TRACE CALLBACK
    # --------------------------------------------------

        # --------------------------------------------------
    # TRACE CALLBACK
    # --------------------------------------------------

    def tracer(frame, event, arg):

        nonlocal limit_reached

        # Trace only submitted Python source code.
        if frame.f_code.co_filename != "<user_code>":
            return None

        if limit_reached is not None:
            raise TraceLimitExceeded()

        if time.monotonic() >= deadline:
            limit_reached = "maximum_trace_time_exceeded"
            raise TraceLimitExceeded()

        if len(events) >= MAX_TRACE_STEPS:

            limit_reached = "maximum_trace_steps_exceeded"

            raise TraceLimitExceeded()

        frame_id = id(frame)
        function_name = frame.f_code.co_name

        # ----------------------------------------------
        # FUNCTION CALL
        # ----------------------------------------------

        if event == "call":

            if len(call_stack) - 1 >= MAX_CALL_DEPTH:

                limit_reached = "maximum_call_depth_exceeded"

                raise TraceLimitExceeded()

            frame_states[frame_id] = {}

            call_stack.append(function_name)

            record_event({
                "step": len(events) + 1,
                "line": frame.f_lineno,
                "event": "function_call",
                "function": function_name,
                "depth": len(call_stack) - 1,
                "call_stack": call_stack.copy(),
                "arguments": capture_variables(frame)
            }, frame)

            return tracer

        # ----------------------------------------------
        # LINE EXECUTION
        # ----------------------------------------------

        if event == "line":

            previous_variables = frame_states.setdefault(
                frame_id, {}
            )

            current_variables = capture_variables(frame)

            changes = detect_changes(
                previous_variables,
                current_variables
            )

            record_event({
                "step": len(events) + 1,
                "line": frame.f_lineno,
                "event": "line",
                "function": function_name,
                "depth": max(0, len(call_stack) - 1),
                "call_stack": call_stack.copy(),
                "variables": current_variables,
                "changes": changes
            }, frame)

            frame_states[frame_id] = current_variables

        # ----------------------------------------------
        # EXCEPTION EVENT
        # ----------------------------------------------

        elif event == "exception":

            exception_type, exception_value, _ = arg

            current_variables = capture_variables(frame)

            record_event({
                "step": len(events) + 1,
                "line": frame.f_lineno,
                "event": "exception",
                "function": function_name,
                "depth": max(0, len(call_stack) - 1),
                "call_stack": call_stack.copy(),
                "variables": current_variables,
                "exception": {
                    "type": exception_type.__name__,
                    "message": exception_message(
                        exception_value
                    )
                }
            }, frame)

        # ----------------------------------------------
        # FUNCTION RETURN
        # ----------------------------------------------

        elif event == "return":

            previous_variables = frame_states.get(
                frame_id, {}
            )

            current_variables = capture_variables(frame)

            changes = detect_changes(
                previous_variables,
                current_variables
            )

            record_event({
                "step": len(events) + 1,
                "line": frame.f_lineno,
                "event": "function_return",
                "function": function_name,
                "depth": max(0, len(call_stack) - 1),
                "call_stack": call_stack.copy(),
                "return_value": serialize_value(arg),
                "variables": current_variables,
                "changes": changes
            }, frame)

            frame_states.pop(frame_id, None)

            if call_stack:
                call_stack.pop()

        return tracer

    # --------------------------------------------------
    # COMPILE AND EXECUTE USER CODE
    # --------------------------------------------------

    execution_globals = {
        "__builtins__": __builtins__
    }

    previous_trace = sys.gettrace()

    try:

        compiled_code = compile(
            code,
            "<user_code>",
            "exec"
        )

        sys.settrace(tracer)

        exec(
            compiled_code,
            execution_globals,
            execution_globals
        )

    except TraceLimitExceeded:

        # Execution was stopped by a configured trace limit.
        pass

    except BaseException as exception:

        execution_error = build_error(exception)

    finally:

        sys.settrace(previous_trace)

    # --------------------------------------------------
    # DETERMINE FINAL STATUS
    # --------------------------------------------------

    if limit_reached is not None:
        status = "limit_reached"

    elif execution_error is not None:
        status = "failed"

    else:
        status = "completed"

    # --------------------------------------------------
    # FINAL RESULT
    # --------------------------------------------------

    return {
        "status": status,
        "limit": limit_reached,
        "error": execution_error,
        "total_steps": len(events),
        "trace_size_bytes": trace_bytes,
        "events": events
    }


# --------------------------------------------------
# MANUAL TESTS
# --------------------------------------------------

if __name__ == "__main__":

    test_cases = {

        "Normal execution": """
x = 10
y = 20
result = x + y
""",

        "ZeroDivisionError": """
x = 10
y = 0
result = x / y
""",

        "IndexError": """
arr = [10, 20, 30]
result = arr[100]
""",

        "NameError": """
x = undefined_variable
""",

        "SyntaxError": """
if x == 5
    print(x)
""",

        "Handled exception": """
try:
    result = 10 / 0
except ZeroDivisionError:
    result = 0
""",

        "Nested function exception": """
def divide(a, b):
    return a / b

def calculate():
    return divide(10, 0)

result = calculate()
""",

        "Maximum trace steps": """
x = 0
while True:
    x += 1
""",

        "Maximum call depth": """
def recurse(n):
    return recurse(n + 1)

recurse(0)
""",

        "Large values": """
arr = list(range(100000))
message = "A" * 200000
result = len(arr)
""",

        "Maximum trace size": """
message = "A" * 60000

for i in range(1000):
    x = i
"""
    }

    for name, code in test_cases.items():

        result = trace_execution(code)

        exception_events = [
            event
            for event in result["events"]
            if event["event"] == "exception"
        ]

        print(f"\n--- {name} ---")

        print(json.dumps({
            "status": result["status"],
            "limit": result["limit"],
            "error": result["error"],
            "total_steps": result["total_steps"],
            "trace_size_bytes": result["trace_size_bytes"],
            "exception_events": len(exception_events)
        }, indent=2))
