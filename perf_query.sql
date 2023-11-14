CALL insert_sparkplug_payload(
        'testgrp',
        'NDATA',
        'testnode',
        'testdevice',
        now(),
        1,
        'c79f4e26-37de-46a1-8820-1f626793961e',
        null,
        ARRAY[
            ROW('string', 123, now(), 12, false, false, false, null, null, 'testval', null, null, null, null, null)::metric_type,
            ROW('bool', 123, now(), 11, false, false, false, null, null, null, true, null, null, null, null)::metric_type,
            ROW('int32', 123, now(), 2, false, false, false, null, null, null, null, 33, null, null, null)::metric_type,
            ROW('uint64', 123, now(), 8, false, false, false, null, null, null, null, 66666666, null, null, null)::metric_type,
            ROW('double', 123, now(), 10, false, false, false, null, null, null, null, null, null, 999.234324, null)::metric_type,
            ROW('float', 123, now(), 9, false, false, false, null, null, null, null, null, null, null, random())::metric_type
            ]
     )


