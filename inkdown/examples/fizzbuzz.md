# FizzBuzz

The mandatory interview question, as a literate program. For the numbers 1
through 15: multiples of three print `Fizz`, multiples of five print `Buzz`,
multiples of both print `FizzBuzz`, and everything else prints the number.

The `%` operator is integer remainder, and `if / else if / else` chains work
the way you expect:

```inkdown
for n in range(1, 16) {
  if n % 15 == 0 {
    print("FizzBuzz")
  } else if n % 3 == 0 {
    print("Fizz")
  } else if n % 5 == 0 {
    print("Buzz")
  } else {
    print(n)
  }
}
```
