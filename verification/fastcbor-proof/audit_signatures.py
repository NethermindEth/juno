"""Extract Lean theorem signatures for explicit-premise reporting."""
import re


def theorem_signatures(source):
    """Read declarations through their top-level :=, regardless of proof style.

    Named arguments also contain :=, so track delimiters and skip Lean strings
    and nested comments instead of searching for a particular proof-body token.
    """
    declarations = list(re.finditer(r"^theorem (\w+)\b", source, re.M))
    for index, declaration in enumerate(declarations):
        start = declaration.end()
        limit = declarations[index + 1].start() if index + 1 < len(declarations) else len(source)
        cursor = start
        stack = []
        closing = {"(": ")", "[": "]", "{": "}", "⦃": "⦄", "⟨": "⟩"}
        while cursor < limit:
            if source.startswith("--", cursor):
                newline = source.find("\n", cursor + 2, limit)
                cursor = limit if newline < 0 else newline + 1
                continue
            if source.startswith("/-", cursor):
                nesting = 1
                cursor += 2
                while cursor < limit and nesting:
                    if source.startswith("/-", cursor):
                        nesting += 1
                        cursor += 2
                    elif source.startswith("-/", cursor):
                        nesting -= 1
                        cursor += 2
                    else:
                        cursor += 1
                if nesting:
                    raise ValueError(f"unterminated comment in theorem {declaration[1]}")
                continue
            char = source[cursor]
            if char == '"':
                cursor += 1
                while cursor < limit and source[cursor] != '"':
                    cursor += 2 if source[cursor] == "\\" else 1
                if cursor >= limit:
                    raise ValueError(f"unterminated string in theorem {declaration[1]}")
                cursor += 1
                continue
            if char in closing:
                stack.append(closing[char])
            elif char in closing.values():
                if not stack or stack.pop() != char:
                    raise ValueError(f"unbalanced signature in theorem {declaration[1]}")
            elif not stack and source.startswith(":=", cursor):
                yield declaration[1], source[start:cursor]
                break
            cursor += 1
        else:
            raise ValueError(f"no proof assignment found for theorem {declaration[1]}")
