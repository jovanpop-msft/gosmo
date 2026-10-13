-- major 13

SELECT SUSER_SNAME(d.owner_sid), d.page_verify_option_desc, d.user_access_desc,
       d.containment_desc, d.is_local_cursor_default, d.snapshot_isolation_state_desc,
       d.is_auto_close_on, d.is_auto_shrink_on, d.is_auto_create_stats_on,
       d.is_auto_update_stats_on, d.is_auto_update_stats_async_on,
       d.is_ansi_null_default_on, d.is_ansi_nulls_on, d.is_ansi_padding_on,
       d.is_ansi_warnings_on, d.is_arithabort_on, d.is_concat_null_yields_null_on,
       d.is_numeric_roundabort_on, d.is_quoted_identifier_on, d.is_recursive_triggers_on,
       d.is_cursor_close_on_commit_on, d.is_read_committed_snapshot_on,
       d.is_trustworthy_on, d.is_broker_enabled, d.is_encrypted,
       ISNULL(fo.non_transacted_access_desc, 'OFF'), ISNULL(fo.directory_name, ''),
       d.is_db_chaining_on, d.is_parameterization_forced, d.is_date_correlation_on,
       d.is_honor_broker_priority_on, d.delayed_durability_desc,
       ISNULL(d.target_recovery_time_in_seconds, -1),
       CAST(0 AS bit),
       CAST(0 AS bit)
FROM   sys.databases d
LEFT   JOIN sys.database_filestream_options fo ON fo.database_id = d.database_id
WHERE  d.name = @p1
-- major 14

SELECT SUSER_SNAME(d.owner_sid), d.page_verify_option_desc, d.user_access_desc,
       d.containment_desc, d.is_local_cursor_default, d.snapshot_isolation_state_desc,
       d.is_auto_close_on, d.is_auto_shrink_on, d.is_auto_create_stats_on,
       d.is_auto_update_stats_on, d.is_auto_update_stats_async_on,
       d.is_ansi_null_default_on, d.is_ansi_nulls_on, d.is_ansi_padding_on,
       d.is_ansi_warnings_on, d.is_arithabort_on, d.is_concat_null_yields_null_on,
       d.is_numeric_roundabort_on, d.is_quoted_identifier_on, d.is_recursive_triggers_on,
       d.is_cursor_close_on_commit_on, d.is_read_committed_snapshot_on,
       d.is_trustworthy_on, d.is_broker_enabled, d.is_encrypted,
       ISNULL(fo.non_transacted_access_desc, 'OFF'), ISNULL(fo.directory_name, ''),
       d.is_db_chaining_on, d.is_parameterization_forced, d.is_date_correlation_on,
       d.is_honor_broker_priority_on, d.delayed_durability_desc,
       ISNULL(d.target_recovery_time_in_seconds, -1),
       CAST(0 AS bit),
       CAST(0 AS bit)
FROM   sys.databases d
LEFT   JOIN sys.database_filestream_options fo ON fo.database_id = d.database_id
WHERE  d.name = @p1
-- major 15

SELECT SUSER_SNAME(d.owner_sid), d.page_verify_option_desc, d.user_access_desc,
       d.containment_desc, d.is_local_cursor_default, d.snapshot_isolation_state_desc,
       d.is_auto_close_on, d.is_auto_shrink_on, d.is_auto_create_stats_on,
       d.is_auto_update_stats_on, d.is_auto_update_stats_async_on,
       d.is_ansi_null_default_on, d.is_ansi_nulls_on, d.is_ansi_padding_on,
       d.is_ansi_warnings_on, d.is_arithabort_on, d.is_concat_null_yields_null_on,
       d.is_numeric_roundabort_on, d.is_quoted_identifier_on, d.is_recursive_triggers_on,
       d.is_cursor_close_on_commit_on, d.is_read_committed_snapshot_on,
       d.is_trustworthy_on, d.is_broker_enabled, d.is_encrypted,
       ISNULL(fo.non_transacted_access_desc, 'OFF'), ISNULL(fo.directory_name, ''),
       d.is_db_chaining_on, d.is_parameterization_forced, d.is_date_correlation_on,
       d.is_honor_broker_priority_on, d.delayed_durability_desc,
       ISNULL(d.target_recovery_time_in_seconds, -1),
       d.is_accelerated_database_recovery_on,
       CAST(0 AS bit)
FROM   sys.databases d
LEFT   JOIN sys.database_filestream_options fo ON fo.database_id = d.database_id
WHERE  d.name = @p1
-- major 16

SELECT SUSER_SNAME(d.owner_sid), d.page_verify_option_desc, d.user_access_desc,
       d.containment_desc, d.is_local_cursor_default, d.snapshot_isolation_state_desc,
       d.is_auto_close_on, d.is_auto_shrink_on, d.is_auto_create_stats_on,
       d.is_auto_update_stats_on, d.is_auto_update_stats_async_on,
       d.is_ansi_null_default_on, d.is_ansi_nulls_on, d.is_ansi_padding_on,
       d.is_ansi_warnings_on, d.is_arithabort_on, d.is_concat_null_yields_null_on,
       d.is_numeric_roundabort_on, d.is_quoted_identifier_on, d.is_recursive_triggers_on,
       d.is_cursor_close_on_commit_on, d.is_read_committed_snapshot_on,
       d.is_trustworthy_on, d.is_broker_enabled, d.is_encrypted,
       ISNULL(fo.non_transacted_access_desc, 'OFF'), ISNULL(fo.directory_name, ''),
       d.is_db_chaining_on, d.is_parameterization_forced, d.is_date_correlation_on,
       d.is_honor_broker_priority_on, d.delayed_durability_desc,
       ISNULL(d.target_recovery_time_in_seconds, -1),
       d.is_accelerated_database_recovery_on,
       d.is_ledger_on
FROM   sys.databases d
LEFT   JOIN sys.database_filestream_options fo ON fo.database_id = d.database_id
WHERE  d.name = @p1
-- major 17

SELECT SUSER_SNAME(d.owner_sid), d.page_verify_option_desc, d.user_access_desc,
       d.containment_desc, d.is_local_cursor_default, d.snapshot_isolation_state_desc,
       d.is_auto_close_on, d.is_auto_shrink_on, d.is_auto_create_stats_on,
       d.is_auto_update_stats_on, d.is_auto_update_stats_async_on,
       d.is_ansi_null_default_on, d.is_ansi_nulls_on, d.is_ansi_padding_on,
       d.is_ansi_warnings_on, d.is_arithabort_on, d.is_concat_null_yields_null_on,
       d.is_numeric_roundabort_on, d.is_quoted_identifier_on, d.is_recursive_triggers_on,
       d.is_cursor_close_on_commit_on, d.is_read_committed_snapshot_on,
       d.is_trustworthy_on, d.is_broker_enabled, d.is_encrypted,
       ISNULL(fo.non_transacted_access_desc, 'OFF'), ISNULL(fo.directory_name, ''),
       d.is_db_chaining_on, d.is_parameterization_forced, d.is_date_correlation_on,
       d.is_honor_broker_priority_on, d.delayed_durability_desc,
       ISNULL(d.target_recovery_time_in_seconds, -1),
       d.is_accelerated_database_recovery_on,
       d.is_ledger_on
FROM   sys.databases d
LEFT   JOIN sys.database_filestream_options fo ON fo.database_id = d.database_id
WHERE  d.name = @p1
