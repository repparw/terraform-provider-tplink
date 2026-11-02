"""Shared runtime helpers with no router-specific protocol duplication."""


def provider():
    from pyvider.hub import hub
    instance = hub.get_component("singleton", "provider")
    if instance is None:
        raise RuntimeError("TP-Link provider has not been configured")
    return instance


def matching_forward(rules, name):
    """Select a uniquely named forward, failing closed on duplicates."""
    matches = [rule for rule in rules if rule.name == name]
    if len(matches) > 1:
        raise RuntimeError("ambiguous duplicate port-forward name; refusing write")
    return matches[0] if matches else None
