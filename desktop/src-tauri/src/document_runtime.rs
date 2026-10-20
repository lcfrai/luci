/// Release tooling validates and stages the originals before supplying these
/// pins at compile time. An installed application never trusts replacement
/// runtime environment variables to choose its document companion.
pub struct DocumentRuntimePins<'a> {
    pub profile: &'a str,
    pub tool_domain: &'a str,
    pub image_manifest: &'a str,
}

impl<'a> DocumentRuntimePins<'a> {
    pub fn validate(
        profile: &'a str,
        tool_domain: &'a str,
        image_manifest: &'a str,
        required: &str,
    ) -> Result<Self, &'static str> {
        if !matches!(required, "" | "0" | "1") {
            return Err("document bundle requirement must be 0 or 1");
        }
        for pin in [profile, tool_domain, image_manifest] {
            if !pin.is_empty()
                && (pin.len() != 64
                    || !pin.bytes().all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte)))
            {
                return Err("document pins must be lowercase SHA-256 values");
            }
        }
        if profile.is_empty() != tool_domain.is_empty() {
            return Err("document profile and tool-domain pins must be supplied together");
        }
        if required == "1" && image_manifest.is_empty() {
            return Err("a full release requires its verified document image companion");
        }
        Ok(Self { profile, tool_domain, image_manifest })
    }
}

pub fn bundled_environment() -> [(&'static str, &'static str); 3] {
    [
        ("LUCI_DOCUMENT_PROFILE_SHA256", option_env!("LUCI_BUNDLED_DOCUMENT_PROFILE_SHA256").unwrap_or("")),
        ("LUCI_DOCUMENT_TOOL_DOMAIN_SHA256", option_env!("LUCI_BUNDLED_DOCUMENT_TOOL_DOMAIN_SHA256").unwrap_or("")),
        ("LUCI_DOCUMENT_IMAGE_MANIFEST_SHA256", option_env!("LUCI_BUNDLED_DOCUMENT_IMAGE_MANIFEST_SHA256").unwrap_or("")),
    ]
}

#[cfg(test)]
mod tests {
    use super::DocumentRuntimePins;

    const PIN: &str = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef";

    #[test]
    fn full_release_requires_image_but_not_windows_producer() {
        assert!(DocumentRuntimePins::validate("", "", PIN, "1").is_ok());
        assert!(DocumentRuntimePins::validate(PIN, PIN, "", "1").is_err());
        assert!(DocumentRuntimePins::validate(PIN, PIN, PIN, "1").is_ok());
    }

    #[test]
    fn developer_omission_does_not_allow_partial_or_malformed_pins() {
        assert!(DocumentRuntimePins::validate("", "", "", "").is_ok());
        assert!(DocumentRuntimePins::validate("", "", "", "0").is_ok());
        assert!(DocumentRuntimePins::validate(PIN, "", PIN, "0").is_err());
        assert!(DocumentRuntimePins::validate("", PIN, PIN, "0").is_err());
        for malformed in [PIN.to_uppercase(), format!("{PIN}\n"), "f".repeat(63), "g".repeat(64)] {
            assert!(DocumentRuntimePins::validate("", "", &malformed, "0").is_err());
        }
        assert!(DocumentRuntimePins::validate("", "", PIN, "true").is_err());
    }
}
