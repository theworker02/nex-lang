# Control flow

## `if` / `else`

```nex
if (cond) {
  puts("yes");
} else {
  puts("no");
};
```

Conditions use truthiness: `false` and `null` are falsy; other values are truthy.

Sugar form:

```nex
if x > 0 then
  puts("pos")
else then
  puts("non-pos")
end
```

## `while`

```nex
let i = 0;
while (i < 3) {
  puts(i);
  i = i + 1;
};
```

## `break` / `continue`

Supported inside loops on the tree-walk and bytecode engines.

## `for` / `in`

Tokenized and partially supported. Prefer `while` for portable code until `for` coverage is complete across engines. Range sugar (`for i in a to b do ... end`) is rewritten experimentally by syntax lowering — verify with the evaluator before relying on it.

## `return`

Exits the current function with a value:

```nex
let abs = fn(n) {
  if (n < 0) {
    return 0 - n;
  };
  return n;
};
```

## Expression statements

The last expression in a script (or REPL line) is the program result. Many demos end with a sentinel like `true` or `"ok"`.
