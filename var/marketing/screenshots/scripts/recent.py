"""Opens a few records first so Home's Recently Viewed list is populated."""
import time

RECORDS = ["table/equipment/4", "table/jobs/7", "table/clients/2", "table/jobs/5", "table/clients/1"]


def run(driver):
    home = driver.current_url
    base = home.split("/", 3)[:3]
    for path in RECORDS:
        driver.get("/".join(base) + "/" + path)
        time.sleep(1.5)
    driver.get(home)
    time.sleep(3)
