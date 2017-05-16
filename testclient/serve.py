import requests
import json
import time
from datetime import datetime, timedelta
from random import randint

KEY = 'd9b2591e-e0be-4958-8d47-950927ebf64f'

URL = 'http://localhost:8080'


def send_measurement(name, value, name2, value2, name3, value3):
    data = [
        {
            'key': name,
            'value': value,
            'timestamp': int(time.mktime((datetime.now() + timedelta(hours=2)).timetuple()))
        },
        {
            'key': name2,
            'value': value2,
            'timestamp': int(time.mktime((datetime.now() + timedelta(hours=2)).timetuple()))
        },
        {
            'key': name3,
            'value': value3,
            'timestamp': int(time.mktime((datetime.now() + timedelta(hours=2)).timetuple()))
        }
    ]
    print data
    r = requests.post('%s/measurements/' % URL, data=json.dumps(data), headers={'X-PYTILT-KEY': KEY})
    print r


if __name__ == '__main__':
    #send_measurement('temp', 1, 'gravity', 2)

    while True:
        send_measurement('room_temp', randint(18, 25), 'tilt_black_temp', randint(19, 30), 'tilt_black_grav', randint(1010, 1090))
        time.sleep(10)
   
