import unittest
import urllib.parse

import msstore


class TokenRequest(unittest.TestCase):
    def test_client_credentials_against_the_tenant_for_the_dev_center_resource(self):
        req = msstore.token_request("tenant-guid", "client-guid", "the-secret")
        self.assertEqual(req.full_url, "https://login.microsoftonline.com/tenant-guid/oauth2/token")
        self.assertEqual(req.get_method(), "POST")
        body = dict(urllib.parse.parse_qsl(req.data.decode()))
        self.assertEqual(body, {
            "grant_type": "client_credentials",
            "client_id": "client-guid",
            "client_secret": "the-secret",
            "resource": "https://manage.devcenter.microsoft.com",
        })

    def test_missing_credentials_name_the_env_script(self):
        with self.assertRaises(SystemExit) as stop:
            msstore.credentials({"MSSTORE_TENANT_ID": "t"})
        self.assertIn("MSSTORE_CLIENT_ID", str(stop.exception))
        self.assertIn("msstore-env.sh", str(stop.exception))


class ApiRequest(unittest.TestCase):
    def test_bearer_token_on_the_my_endpoint(self):
        req = msstore.api_request("/applications?top=50", "tok")
        self.assertEqual(req.full_url, "https://manage.devcenter.microsoft.com/v1.0/my/applications?top=50")
        self.assertEqual(req.get_header("Authorization"), "Bearer tok")

    def test_relative_paths_are_refused(self):
        with self.assertRaises(ValueError):
            msstore.api_request("applications", "tok")


class AppListing(unittest.TestCase):
    def test_summary_keeps_the_ids_a_release_needs(self):
        listing = {"value": [
            {"id": "9NDCCZH9RB9K", "primaryName": "BibleText",
             "pendingApplicationSubmission": {"id": "1152921505695"},
             "lastPublishedApplicationSubmission": None},
            {"id": "9NABC", "primaryName": "Other"},
        ]}
        self.assertEqual(msstore.summarise_apps(listing), [
            {"id": "9NDCCZH9RB9K", "name": "BibleText", "pendingSubmission": "1152921505695",
             "lastPublishedSubmission": None},
            {"id": "9NABC", "name": "Other", "pendingSubmission": None, "lastPublishedSubmission": None},
        ])


if __name__ == "__main__":
    unittest.main()
