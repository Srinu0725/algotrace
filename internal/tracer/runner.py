
import sys
import json
import io

from contextlib import redirect_stdout, redirect_stderr

from tracer import trace_execution


# --------------------------------------------------
# OUTPUT SAFETY LIMITS
# --------------------------------------------------

MAX_OUTPUT_BYTES = 1024 * 1024


# --------------------------------------------------
# BOUNDED OUTPUT BUFFER
# --------------------------------------------------

class BoundedOutputBuffer(io.TextIOBase):

    def __init__(self, max_bytes=MAX_OUTPUT_BYTES):

        super().__init__()

        self.max_bytes = max_bytes
        self.buffer = bytearray()
        self.truncated = False

    def writable(self):

        return True

    def write(self, text):

        if not isinstance(text, str):
            raise TypeError("Expected a string")

        remaining = self.max_bytes - len(self.buffer)

        if remaining <= 0:

            if text:
                self.truncated = True

            return len(text)

        # Bound the amount of text encoded at once.
        encoded = text[:remaining].encode(
            "utf-8",
            errors="replace"
        )

        if len(text) > remaining or len(encoded) > remaining:
            self.truncated = True

        # Preserve valid UTF-8 when truncating.
        safe_bytes = (
            encoded[:remaining]
            .decode("utf-8", errors="ignore")
            .encode("utf-8")
        )

        self.buffer.extend(safe_bytes)

        return len(text)

    def getvalue(self):

        return self.buffer.decode("utf-8")

    def flush(self):

        pass


# --------------------------------------------------
# EXECUTE USER CODE
# --------------------------------------------------

def execute(code):

    stdout_buffer = BoundedOutputBuffer()
    stderr_buffer = BoundedOutputBuffer()

    with redirect_stdout(stdout_buffer):

        with redirect_stderr(stderr_buffer):

            result = trace_execution(code)

    result["stdout"] = stdout_buffer.getvalue()
    result["stderr"] = stderr_buffer.getvalue()

    result["stdout_truncated"] = stdout_buffer.truncated
    result["stderr_truncated"] = stderr_buffer.truncated

    return result


# --------------------------------------------------
# RUNNER ENTRY POINT
# --------------------------------------------------

def main():

    try:

        code = sys.stdin.read()

        result = execute(code)

        print(json.dumps(
            result,
            ensure_ascii=True,
            allow_nan=False,
            separators=(",", ":")
        ))

    except Exception as exception:

        print(json.dumps({
            "status": "failed",
            "limit": None,
            "error": {
                "type": type(exception).__name__,
                "message": str(exception)[:4096],
                "line": None,
                "function": None
            },
            "stdout": "",
            "stderr": "",
            "stdout_truncated": False,
            "stderr_truncated": False,
            "total_steps": 0,
            "trace_size_bytes": 0,
            "events": []
        }))


if __name__ == "__main__":

    main()
