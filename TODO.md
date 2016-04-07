
## Bugs

+ .load appends the history to the history file but should actually use readline save history function line by line instead
+ otto is missing Number.parseInt() see https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/parseInt for polyfill

## Someday Maybe

+ Make sure I can run JS generated from GopherJS
    + Support typed arrays (e.g. Unit8Array)
