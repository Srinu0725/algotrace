import json
import math
import types

from collections import deque



MAX_OBJECTS = 100
MAX_TRACKED_IDENTITIES = 2000
MAX_ATTRIBUTES = 40
MAX_ROOTS = 100
MAX_SNAPSHOT_BYTES = 64 * 1024
MAX_STRING_LENGTH = 256


class ObjectTracker:

    def __init__(self):

        self.identities = {}
        self.next_id = 1

    def get_object_id(self, value):

        identity = id(value)

        existing = self.identities.get(identity)

        if existing is not None:
            return existing[0]

        if len(self.identities) >= MAX_TRACKED_IDENTITIES:
            return None

        object_id = f"obj_{self.next_id}"

        self.next_id += 1

        # Keep a strong reference to prevent Python
        # from reusing this object's identity.
        self.identities[identity] = (
            object_id,
            value
        )

        return object_id

    def get_attributes(self, value):

        value_type = type(value)

        if value_type is list:
            return {
                str(index): item
                for index, item in enumerate(
                    value[:MAX_ATTRIBUTES]
                )
            }

        if value_type is dict:

            attributes = {}

            for index, (key, item) in enumerate(
                value.items()
            ):

                if index >= MAX_ATTRIBUTES:
                    break

                if type(key) is str:
                    label = key[:MAX_STRING_LENGTH]

                elif type(key) is int and key.bit_length() <= 128:
                    label = str(key)

                else:
                    label = f"key_{index}"

                # Avoid collisions between display keys.
                if label in attributes:
                    label = f"{label}_{index}"

                attributes[label] = item

            return attributes

        # Exclude classes, modules and functions.
        if isinstance(value, (
            type,
            types.ModuleType,
            types.FunctionType,
            types.MethodType,
            types.BuiltinFunctionType
        )):
            return None

        # Only inspect ordinary instance dictionaries.
        # Avoid arbitrary properties named __dict__.
        try:

            mro = type.__getattribute__(
                value_type, "__mro__"
            )

            for base in mro:

                namespace = type.__getattribute__(
                    base, "__dict__"
                )

                descriptor = namespace.get("__dict__")

                if descriptor is None:
                    continue

                if type(descriptor) is not types.GetSetDescriptorType:
                    return None

                attributes = descriptor.__get__(
                    value, value_type
                )

                if type(attributes) is not dict:
                    return None

                result = {}

                for index, (key, item) in enumerate(
                    attributes.items()
                ):

                    if index >= MAX_ATTRIBUTES:
                        break

                    if type(key) is str:
                        result[key[:MAX_STRING_LENGTH]] = item

                return result

        except Exception:
            return None

        return None

    def snapshot(self, variables):

        objects = {}
        references = {}

        pending = deque()
        scheduled = set()

        truncated = False

        def encode(value):

            nonlocal truncated

            value_type = type(value)

            if value is None or value_type is bool:
                return value

            if value_type is int:

                if value.bit_length() > 128:
                    return "<large integer>"

                return value

            if value_type is float:
                return (
                    value
                    if math.isfinite(value)
                    else str(value)
                )

            if value_type is str:
                return value[:MAX_STRING_LENGTH]

            attributes = self.get_attributes(value)

            if attributes is None:
                return "<unsupported value>"

            object_id = self.get_object_id(value)

            if object_id is None:

                truncated = True

                return {"$truncated": True}

            if object_id not in scheduled:

                if len(scheduled) >= MAX_OBJECTS:

                    truncated = True

                    return {"$truncated": True}

                scheduled.add(object_id)

                pending.append((
                    object_id,
                    value,
                    attributes
                ))

            return {"$ref": object_id}

        for index, (name, value) in enumerate(
            variables.items()
        ):

            if index >= MAX_ROOTS:

                truncated = True

                break

            if name.startswith("__"):
                continue

            if self.get_attributes(value) is None:
                continue

            encoded = encode(value)

            if (
                type(encoded) is dict
                and "$ref" in encoded
            ):

                references[name[:MAX_STRING_LENGTH]] = (
                    encoded["$ref"]
                )

        # Breadth-first traversal.
        # The scheduled set prevents infinite cycles.
        while pending:

            object_id, value, attributes = (
                pending.popleft()
            )

            type_name = type.__getattribute__(
                type(value), "__name__"
            )

            objects[object_id] = {
                "type": type_name[:MAX_STRING_LENGTH],
                "attributes": {
                    key: encode(item)
                    for key, item in attributes.items()
                }
            }

        result = {
            "references": references,
            "objects": objects,
            "objects_truncated": truncated
        }

        encoded_result = json.dumps(
            result,
            ensure_ascii=True,
            allow_nan=False,
            separators=(",", ":")
        ).encode("utf-8")

        if len(encoded_result) > MAX_SNAPSHOT_BYTES:

            return {
                "references": {},
                "objects": {},
                "objects_truncated": True
            }

        return result