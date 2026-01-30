# for i in {1..50}; do
# 	echo "=== Iteration $i ===" | tee -a cc_run_log.txt
# 	make test_base_with_claude_code 2>&1 | tee -a cc_run_log.txt
# done

for i in {1..50}; do
	echo "=== Iteration $i ===" | tee -a run_log.txt
	make test_base_with_users_code 2>&1 | tee -a run_log.txt
done
