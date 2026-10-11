export function calculate(a, b) {
  const number = (value) => {
    if (typeof value !== 'string' || value.trim() === '' || !/^[+-]?(?:\d+(?:\.\d*)?|\.\d+)$/.test(value.trim())) throw new TypeError('Enter two finite decimal amounts');
    const parsed = Number(value);
    if (!Number.isFinite(parsed)) throw new TypeError('Enter two finite decimal amounts');
    return parsed;
  };
  const sum = number(a) + number(b);
  if (!Number.isFinite(sum)) throw new TypeError('The sum must be finite');
  return sum;
}
