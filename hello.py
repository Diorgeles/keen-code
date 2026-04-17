def fibonacci(n):
    if n <= 1:
        return n
    a, b = 0, 1
    for _ in range(2, n + 1):
        a, b = b, a + b
    return b


def golden_ratio(iterations):
    if iterations < 2:
        raise ValueError("iterations must be at least 2")
    return fibonacci(iterations) / fibonacci(iterations - 1)


print(fibonacci(100))
print(golden_ratio(100))
