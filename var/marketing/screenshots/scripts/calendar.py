"""Switches the table to its Calendar view via the Views menu."""
import time

from selenium.webdriver.common.action_chains import ActionChains
from selenium.webdriver.common.by import By


def run(driver):
    views = [b for b in driver.find_elements(By.CSS_SELECTOR, "main button") if b.text.strip() == "Views"]
    views[0].click()
    time.sleep(0.5)
    driver.find_element(By.XPATH, "//span[contains(@class, 'views-picker-name')][normalize-space()='Calendar']").click()
    # Park the pointer on the section title so no day cell shows a hover state.
    ActionChains(driver).move_to_element(driver.find_element(By.CSS_SELECTOR, "main h2")).perform()
    time.sleep(2)
