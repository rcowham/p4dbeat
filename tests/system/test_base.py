from p4dbeat import BaseTest

import os


class Test(BaseTest):

    def test_base(self):
        """
        Basic test with exiting P4dbeat normally
        """
        self.render_config_template(
            path=os.path.abspath(self.working_dir) + "/log/*"
        )

        p4dbeat_proc = self.start_beat()
        self.wait_until(lambda: self.log_contains("p4dbeat is running"))
        exit_code = p4dbeat_proc.kill_and_wait()
        assert exit_code == 0
