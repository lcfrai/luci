//! UTF-16 command-line construction for CreateProcessW's CRT argument convention.
//! Kept independent of Windows FFI so quoting can be checked on every host.
use std::io;

pub(super) fn command_line(program: &[u16], args: &[Vec<u16>]) -> io::Result<Vec<u16>> {
    if program.is_empty() || program.iter().any(|c| *c == 0 || *c == b'"' as u16) {
        return Err(io::Error::new(io::ErrorKind::InvalidInput, "Invalid sidecar executable path"));
    }
    let mut result = vec![b'"' as u16];
    result.extend_from_slice(program);
    result.push(b'"' as u16);
    for arg in args {
        if arg.contains(&0) {
            return Err(io::Error::new(io::ErrorKind::InvalidInput, "Invalid sidecar argument"));
        }
        result.extend([b' ' as u16, b'"' as u16]);
        let mut slashes = 0;
        for &c in arg {
            if c == b'\\' as u16 {
                slashes += 1;
            } else {
                // Before a quote, every backslash is doubled and the quote is escaped.
                let count = if c == b'"' as u16 { slashes * 2 + 1 } else { slashes };
                result.extend(std::iter::repeat_n(b'\\' as u16, count));
                result.push(c);
                slashes = 0;
            }
        }
        // Trailing backslashes must not escape the closing quote.
        result.extend(std::iter::repeat_n(b'\\' as u16, slashes * 2));
        result.push(b'"' as u16);
    }
    result.push(0);
    Ok(result)
}

#[cfg(test)]
mod tests {
    use super::*;
    fn line(program: &str, args: &[&str]) -> String {
        let mut result = command_line(&program.encode_utf16().collect::<Vec<_>>(), &args.iter().map(|v| v.encode_utf16().collect()).collect::<Vec<_>>()).unwrap();
        assert_eq!(result.pop(), Some(0));
        String::from_utf16(&result).unwrap()
    }
    #[test]
    fn paths_with_spaces_and_unicode_stay_one_executable() {
        assert_eq!(line(r"C:\A B\東京\luci-api.exe", &[]), "\"C:\\A B\\東京\\luci-api.exe\"");
    }
    #[test]
    fn arguments_preserve_empty_space_quote_and_trailing_slash() {
        assert_eq!(line("api.exe", &["", "a b", "a\"b", "end\\"]), "\"api.exe\" \"\" \"a b\" \"a\\\"b\" \"end\\\\\"");
        assert_eq!(line("api.exe", &["slash\\\"quote"]), "\"api.exe\" \"slash\\\\\\\"quote\"");
    }
    #[test]
    fn invalid_program_or_nul_argument_is_rejected_before_creation() {
        for path in [vec![], vec![0], vec![b'"' as u16]] {
            assert!(command_line(&path, &[]).is_err());
        }
        assert!(command_line(&[b'a' as u16], &[vec![0]]).is_err());
    }
    #[test]
    fn unpaired_utf16_units_are_not_lossily_reencoded() {
        assert_eq!(command_line(&[0xd800], &[vec![0xdc00]]).unwrap(), vec![34, 0xd800, 34, 32, 34, 0xdc00, 34, 0]);
    }
}
