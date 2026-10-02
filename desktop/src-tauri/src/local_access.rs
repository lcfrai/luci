//! Account access for the desktop's own local service. The capability never
//! enters the WebView, runtime_info, a file, or an external LUCI_API request.
use serde::{de::DeserializeOwned, Deserialize, Serialize};
use std::{
    io::{Read, Write},
    net::{SocketAddr, TcpStream},
    time::Duration,
};

const RESPONSE_LIMIT: usize = 64 * 1024;

pub struct Connection {
    pub address: SocketAddr,
    pub token: String,
}

#[derive(Deserialize, Serialize)]
#[serde(rename_all = "camelCase")]
pub struct User {
    pub id: String,
    pub username: String,
    pub display_name: String,
    pub role: String,
    pub disabled: bool,
}

#[derive(Deserialize, Serialize)]
pub struct AuthResult {
    pub token: String,
    pub user: User,
}

#[derive(Serialize)]
pub struct Info {
    pub available: bool,
    pub owner: Option<User>,
}

#[derive(Deserialize)]
struct OwnerResponse {
    owner: Option<User>,
}

pub fn info(connection: &Connection) -> Result<Info, String> {
    let response: OwnerResponse = request(connection, "GET", "/v1/auth/local-owner", None)?;
    Ok(Info {
        available: true,
        owner: response.owner,
    })
}

pub fn create(
    connection: &Connection,
    username: &str,
    display_name: &str,
    password: &str,
) -> Result<AuthResult, String> {
    if username.len() > 64 || display_name.len() > 120 || password.len() > 1024 {
        return Err("Use a username up to 64 characters, a name up to 120 characters and a password up to 1,024 characters.".into());
    }
    let body = serde_json::to_vec(&serde_json::json!({
        "username": username,
        "displayName": display_name,
        "password": password,
    }))
    .map_err(|_| "Your account details could not be prepared.")?;
    request(connection, "POST", "/v1/auth/local-account", Some(&body))
}

pub fn recover(connection: &Connection, new_password: &str) -> Result<AuthResult, String> {
    if !(12..=1024).contains(&new_password.len()) {
        return Err("Use a new password of 12–1,024 characters.".into());
    }
    let body = serde_json::to_vec(&serde_json::json!({ "newPassword": new_password }))
        .map_err(|_| "Your new password could not be prepared.")?;
    request(
        connection,
        "POST",
        "/v1/auth/local-owner/recover",
        Some(&body),
    )
}

fn request<T: DeserializeOwned>(
    connection: &Connection,
    method: &str,
    path: &str,
    body: Option<&[u8]>,
) -> Result<T, String> {
    if !connection.address.ip().is_loopback()
        || connection.token.len() < 40
        || !connection
            .token
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric())
    {
        return Err(
            "Account access is available only for Luci's own service on this computer.".into(),
        );
    }
    let body = body.unwrap_or_default();
    let mut stream = TcpStream::connect_timeout(&connection.address, Duration::from_secs(3))
        .map_err(|_| "Luci's local service is unavailable. Retry the connection.")?;
    stream
        .set_read_timeout(Some(Duration::from_secs(30)))
        .map_err(|_| "Luci could not connect to its local service.")?;
    stream
        .set_write_timeout(Some(Duration::from_secs(3)))
        .map_err(|_| "Luci could not connect to its local service.")?;
    // HTTP/1.0 and Connection: close provide bounded, non-chunked response
    // framing without adding an HTTP runtime to this small native bridge.
    let headers = format!(
        "{method} {path} HTTP/1.0\r\nHost: {}\r\nX-Luci-Local-Access-Token: {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        connection.address,
        connection.token,
        body.len()
    );
    stream
        .write_all(headers.as_bytes())
        .and_then(|_| stream.write_all(body))
        .map_err(|_| "Your request did not reach Luci's local service. Retry the connection.")?;
    let mut response = Vec::new();
    stream
        .take((RESPONSE_LIMIT + 1) as u64)
        .read_to_end(&mut response)
        .map_err(|_| {
            "Luci's local service did not finish the request. Check your connection and try again."
        })?;
    decode_response(&response)
}

fn decode_response<T: DeserializeOwned>(response: &[u8]) -> Result<T, String> {
    if response.len() > RESPONSE_LIMIT {
        return Err("Luci's local service returned an invalid account response.".into());
    }
    let boundary = response
        .windows(4)
        .position(|part| part == b"\r\n\r\n")
        .filter(|&position| position <= 8192)
        .ok_or("Luci's local service returned an invalid account response.")?;
    let headers = std::str::from_utf8(&response[..boundary])
        .map_err(|_| "Luci's local service returned an invalid account response.")?;
    let status = headers
        .lines()
        .next()
        .and_then(|line| line.split_whitespace().nth(1))
        .and_then(|code| code.parse::<u16>().ok())
        .ok_or("Luci's local service returned an invalid account response.")?;
    let body = &response[boundary + 4..];
    if !(200..300).contains(&status) {
        // Return only the server's bounded user-facing message, never its
        // headers, request body, passwords, or native capability.
        let message = serde_json::from_slice::<serde_json::Value>(body)
            .ok()
            .and_then(|value| value.get("message").and_then(|v| v.as_str()).map(str::to_owned))
            .filter(|value| !value.is_empty() && value.len() <= 2048)
            .unwrap_or_else(|| "Luci could not complete account access. Retry or contact your workspace administrator.".into());
        return Err(message);
    }
    serde_json::from_slice(body)
        .map_err(|_| "Luci's local service returned an invalid account response.".into())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn account_response_is_typed_and_preserves_identity() {
        let response = b"HTTP/1.0 201 Created\r\nContent-Type: application/json\r\n\r\n{\"token\":\"session\",\"user\":{\"id\":\"same-owner\",\"username\":\"owner\",\"displayName\":\"Owner\",\"role\":\"super-admin\",\"disabled\":false}}";
        let result: AuthResult = decode_response(response).unwrap();
        assert_eq!(result.user.id, "same-owner");
        assert_eq!(result.token, "session");
    }

    #[test]
    fn failure_returns_only_user_facing_message() {
        let response = b"HTTP/1.0 409 Conflict\r\nX-Secret: never-show\r\n\r\n{\"error\":\"username_exists\",\"message\":\"Choose another username.\"}";
        assert_eq!(
            decode_response::<AuthResult>(response).err().unwrap(),
            "Choose another username."
        );
    }

    #[test]
    fn malformed_and_oversized_responses_are_rejected() {
        assert!(decode_response::<AuthResult>(b"not HTTP").is_err());
        assert!(decode_response::<AuthResult>(&vec![b'a'; RESPONSE_LIMIT + 1]).is_err());
        assert!(decode_response::<AuthResult>(b"HTTP/1.0 200 OK\r\n\r\n{}").is_err());
    }

    #[test]
    fn external_address_and_invalid_capability_are_never_contacted() {
        let remote = Connection {
            address: "192.0.2.1:1234".parse().unwrap(),
            token: "a".repeat(64),
        };
        assert!(info(&remote).is_err());
        let malformed = Connection {
            address: "127.0.0.1:1".parse().unwrap(),
            token: "a\r\nHeader: value".into(),
        };
        assert!(info(&malformed).is_err());
    }

    #[test]
    fn bridge_sends_owned_capability_to_loopback_and_returns_only_public_owner() {
        use std::io::BufRead;
        let listener = std::net::TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let token = "a".repeat(64);
        let expected_token = token.clone();
        let server = std::thread::spawn(move || {
            let (stream, _) = listener.accept().unwrap();
            stream
                .set_read_timeout(Some(Duration::from_secs(3)))
                .unwrap();
            let mut reader = std::io::BufReader::new(stream);
            let mut lines = Vec::new();
            loop {
                let mut line = String::new();
                assert!(reader.read_line(&mut line).unwrap() > 0);
                if line == "\r\n" {
                    break;
                }
                lines.push(line);
            }
            assert_eq!(lines[0], "GET /v1/auth/local-owner HTTP/1.0\r\n");
            assert!(lines
                .iter()
                .any(|line| line == &format!("X-Luci-Local-Access-Token: {expected_token}\r\n")));
            reader.get_mut().write_all(b"HTTP/1.0 200 OK\r\nConnection: close\r\n\r\n{\"owner\":{\"id\":\"owner-id\",\"username\":\"owner\",\"displayName\":\"Owner\",\"role\":\"super-admin\",\"disabled\":false}}").unwrap();
        });
        let result = info(&Connection { address, token }).unwrap();
        assert!(result.available);
        assert_eq!(result.owner.unwrap().id, "owner-id");
        server.join().unwrap();
    }
}
