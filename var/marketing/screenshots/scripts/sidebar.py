"""Keeps the sidebar pinned open so the navigation apps are visible."""


def run(driver):
    if driver.execute_script("return localStorage.getItem('picocrank-sidebar-stuck');") != "true":
        driver.execute_script("localStorage.setItem('picocrank-sidebar-stuck', 'true'); location.reload();")
